package runbooks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("runbook not found")

// Status represents the verification status of a runbook
type Status struct {
	DocumentID               uuid.UUID
	TenantID                 uuid.UUID
	LastVerifiedAt           *time.Time
	LastVerifiedByUserID     *uuid.UUID
	VerificationIntervalDays int
	NextDueAt                *time.Time

	// Joined fields
	LastVerifiedBy *UserInfo
}

type UserInfo struct {
	ID    uuid.UUID
	Name  string
	Email string
}

// RunbookWithStatus combines document info with verification status
type RunbookWithStatus struct {
	DocumentID  uuid.UUID
	TenantID    uuid.UUID
	Path        string
	Title       string
	ClientID    *uuid.UUID
	ClientName  *string
	OwnerID     *uuid.UUID
	OwnerName   *string
	UpdatedAt   time.Time
	Status      *Status
	IsOverdue   bool
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetStatus returns the runbook status for a document
func (r *Repository) GetStatus(ctx context.Context, tenantID, documentID uuid.UUID) (*Status, error) {
	var s Status
	var verifierID *uuid.UUID
	var verifierName, verifierEmail *string

	err := r.db.QueryRow(ctx, `
		SELECT rs.document_id, rs.tenant_id, rs.last_verified_at, rs.last_verified_by_user_id,
		       rs.verification_interval_days, rs.next_due_at,
		       u.id, u.name, u.email
		FROM runbook_status rs
		LEFT JOIN users u ON rs.last_verified_by_user_id = u.id
		WHERE rs.tenant_id = $1 AND rs.document_id = $2
	`, tenantID, documentID).Scan(
		&s.DocumentID, &s.TenantID, &s.LastVerifiedAt, &s.LastVerifiedByUserID,
		&s.VerificationIntervalDays, &s.NextDueAt,
		&verifierID, &verifierName, &verifierEmail,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query runbook status: %w", err)
	}

	if verifierID != nil {
		s.LastVerifiedBy = &UserInfo{
			ID:    *verifierID,
			Name:  *verifierName,
			Email: *verifierEmail,
		}
	}

	return &s, nil
}

// EnsureStatus creates a runbook_status record if it doesn't exist
func (r *Repository) EnsureStatus(ctx context.Context, tenantID, documentID uuid.UUID, intervalDays int) error {
	if intervalDays <= 0 {
		intervalDays = 90 // default
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO runbook_status (document_id, tenant_id, verification_interval_days)
		VALUES ($1, $2, $3)
		ON CONFLICT (document_id) DO NOTHING
	`, documentID, tenantID, intervalDays)
	if err != nil {
		return fmt.Errorf("ensure runbook status: %w", err)
	}
	return nil
}

// UpdateInterval updates the verification interval for a runbook
func (r *Repository) UpdateInterval(ctx context.Context, tenantID, documentID uuid.UUID, intervalDays int) error {
	result, err := r.db.Exec(ctx, `
		UPDATE runbook_status
		SET verification_interval_days = $3,
		    next_due_at = CASE 
		        WHEN last_verified_at IS NOT NULL 
		        THEN last_verified_at + ($3::int * INTERVAL '1 day')
		        ELSE NULL
		    END
		WHERE tenant_id = $1 AND document_id = $2
	`, tenantID, documentID, intervalDays)
	if err != nil {
		return fmt.Errorf("update interval: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Verify marks a runbook as verified and computes next_due_at
func (r *Repository) Verify(ctx context.Context, tenantID, documentID uuid.UUID, verifiedBy uuid.UUID) (*Status, error) {
	now := time.Now()

	_, err := r.db.Exec(ctx, `
		UPDATE runbook_status
		SET last_verified_at = $3,
		    last_verified_by_user_id = $4,
		    next_due_at = $3::timestamptz + (verification_interval_days * INTERVAL '1 day')
		WHERE tenant_id = $1 AND document_id = $2
	`, tenantID, documentID, now, verifiedBy)
	if err != nil {
		return nil, fmt.Errorf("verify runbook: %w", err)
	}

	return r.GetStatus(ctx, tenantID, documentID)
}

// ListOverdue returns runbooks that are past their next_due_at
func (r *Repository) ListOverdue(ctx context.Context, tenantID uuid.UUID) ([]RunbookWithStatus, error) {
	return r.listRunbooks(ctx, tenantID, "overdue")
}

// ListUnowned returns runbooks without an owner
func (r *Repository) ListUnowned(ctx context.Context, tenantID uuid.UUID) ([]RunbookWithStatus, error) {
	return r.listRunbooks(ctx, tenantID, "unowned")
}

// ListRecentlyVerified returns recently verified runbooks
func (r *Repository) ListRecentlyVerified(ctx context.Context, tenantID uuid.UUID) ([]RunbookWithStatus, error) {
	return r.listRunbooks(ctx, tenantID, "recent")
}

// ListAll returns all runbooks with their status
func (r *Repository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]RunbookWithStatus, error) {
	return r.listRunbooks(ctx, tenantID, "all")
}

func (r *Repository) listRunbooks(ctx context.Context, tenantID uuid.UUID, filter string) ([]RunbookWithStatus, error) {
	query := `
		SELECT d.id, d.tenant_id, d.path, d.title, d.client_id, c.name AS client_name,
		       d.owner_user_id, u.name AS owner_name, d.updated_at,
		       rs.last_verified_at, rs.last_verified_by_user_id, rs.verification_interval_days, rs.next_due_at,
		       CASE WHEN rs.next_due_at IS NOT NULL AND rs.next_due_at < NOW() THEN true ELSE false END AS is_overdue
		FROM documents d
		LEFT JOIN runbook_status rs ON d.id = rs.document_id
		LEFT JOIN clients c ON d.client_id = c.id
		LEFT JOIN users u ON d.owner_user_id = u.id
		WHERE d.tenant_id = $1 AND d.doc_type = 'runbook'
	`

	switch filter {
	case "overdue":
		query += " AND rs.next_due_at IS NOT NULL AND rs.next_due_at < NOW()"
		query += " ORDER BY rs.next_due_at ASC"
	case "unowned":
		query += " AND d.owner_user_id IS NULL"
		query += " ORDER BY d.updated_at DESC"
	case "recent":
		query += " AND rs.last_verified_at IS NOT NULL"
		query += " ORDER BY rs.last_verified_at DESC LIMIT 20"
	default: // "all"
		query += " ORDER BY d.updated_at DESC"
	}

	rows, err := r.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list runbooks: %w", err)
	}
	defer rows.Close()

	var runbooks []RunbookWithStatus
	for rows.Next() {
		var rb RunbookWithStatus
		var lastVerifiedAt *time.Time
		var lastVerifiedByID *uuid.UUID
		var intervalDays *int
		var nextDueAt *time.Time

		err := rows.Scan(
			&rb.DocumentID, &rb.TenantID, &rb.Path, &rb.Title, &rb.ClientID, &rb.ClientName,
			&rb.OwnerID, &rb.OwnerName, &rb.UpdatedAt,
			&lastVerifiedAt, &lastVerifiedByID, &intervalDays, &nextDueAt,
			&rb.IsOverdue,
		)
		if err != nil {
			return nil, fmt.Errorf("scan runbook: %w", err)
		}

		if lastVerifiedAt != nil || intervalDays != nil {
			rb.Status = &Status{
				DocumentID:               rb.DocumentID,
				TenantID:                 rb.TenantID,
				LastVerifiedAt:           lastVerifiedAt,
				LastVerifiedByUserID:     lastVerifiedByID,
				VerificationIntervalDays: 90, // default
				NextDueAt:                nextDueAt,
			}
			if intervalDays != nil {
				rb.Status.VerificationIntervalDays = *intervalDays
			}
		}

		runbooks = append(runbooks, rb)
	}

	return runbooks, nil
}

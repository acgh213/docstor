// Package changes manages change records — planned changes with risk and status tracking.
package changes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("change not found")

type Change struct {
	ID                     uuid.UUID
	TenantID               uuid.UUID
	ClientID               *uuid.UUID
	Title                  string
	DescriptionMarkdown    string
	RiskLevel              string
	Status                 string
	WindowStart            *time.Time
	WindowEnd              *time.Time
	RollbackPlanMarkdown   string
	ValidationPlanMarkdown string
	CreatedBy              uuid.UUID
	ApprovedBy             *uuid.UUID
	ApprovedAt             *time.Time
	CompletedAt            *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
	// Joined fields
	CreatorName string
	ApproverName string
	ClientName   string
}

type ChangeLink struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	ChangeID   uuid.UUID
	LinkedType string
	LinkedID   uuid.UUID
	LinkedName string // joined
	CreatedAt  time.Time
}

type CreateInput struct {
	TenantID               uuid.UUID
	ClientID               *uuid.UUID
	Title                  string
	DescriptionMarkdown    string
	RiskLevel              string
	WindowStart            *time.Time
	WindowEnd              *time.Time
	RollbackPlanMarkdown   string
	ValidationPlanMarkdown string
	CreatedBy              uuid.UUID
}

type UpdateInput struct {
	Title                  string
	DescriptionMarkdown    string
	ClientID               *uuid.UUID
	RiskLevel              string
	WindowStart            *time.Time
	WindowEnd              *time.Time
	RollbackPlanMarkdown   string
	ValidationPlanMarkdown string
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const changeColumns = `c.id, c.tenant_id, c.client_id, c.title, c.description_markdown,
	c.risk_level, c.status, c.window_start, c.window_end,
	c.rollback_plan_markdown, c.validation_plan_markdown,
	c.created_by, c.approved_by, c.approved_at, c.completed_at,
	c.created_at, c.updated_at,
	COALESCE(u.name, ''), COALESCE(a.name, ''), COALESCE(cl.name, '')`

const changeJoins = `FROM changes c
	LEFT JOIN users u ON c.created_by = u.id
	LEFT JOIN users a ON c.approved_by = a.id
	LEFT JOIN clients cl ON c.client_id = cl.id`

func scanChange(row pgx.Row) (*Change, error) {
	var c Change
	err := row.Scan(
		&c.ID, &c.TenantID, &c.ClientID, &c.Title, &c.DescriptionMarkdown,
		&c.RiskLevel, &c.Status, &c.WindowStart, &c.WindowEnd,
		&c.RollbackPlanMarkdown, &c.ValidationPlanMarkdown,
		&c.CreatedBy, &c.ApprovedBy, &c.ApprovedAt, &c.CompletedAt,
		&c.CreatedAt, &c.UpdatedAt,
		&c.CreatorName, &c.ApproverName, &c.ClientName,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

func (r *Repository) List(ctx context.Context, tenantID uuid.UUID, status string, clientID *uuid.UUID) ([]Change, error) {
	query := `SELECT ` + changeColumns + ` ` + changeJoins + ` WHERE c.tenant_id = $1`
	args := []any{tenantID}
	n := 2

	if status != "" {
		query += fmt.Sprintf(" AND c.status = $%d", n)
		args = append(args, status)
		n++
	}
	if clientID != nil {
		query += fmt.Sprintf(" AND c.client_id = $%d", n)
		args = append(args, *clientID)
		n++
	}

	query += " ORDER BY c.updated_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list changes: %w", err)
	}
	defer rows.Close()

	var changes []Change
	for rows.Next() {
		var c Change
		if err := rows.Scan(
			&c.ID, &c.TenantID, &c.ClientID, &c.Title, &c.DescriptionMarkdown,
			&c.RiskLevel, &c.Status, &c.WindowStart, &c.WindowEnd,
			&c.RollbackPlanMarkdown, &c.ValidationPlanMarkdown,
			&c.CreatedBy, &c.ApprovedBy, &c.ApprovedAt, &c.CompletedAt,
			&c.CreatedAt, &c.UpdatedAt,
			&c.CreatorName, &c.ApproverName, &c.ClientName,
		); err != nil {
			return nil, fmt.Errorf("scan change: %w", err)
		}
		changes = append(changes, c)
	}
	return changes, rows.Err()
}

func (r *Repository) Get(ctx context.Context, tenantID, id uuid.UUID) (*Change, error) {
	query := `SELECT ` + changeColumns + ` ` + changeJoins + ` WHERE c.tenant_id = $1 AND c.id = $2`
	return scanChange(r.db.QueryRow(ctx, query, tenantID, id))
}

func (r *Repository) Create(ctx context.Context, in CreateInput) (*Change, error) {
	query := `INSERT INTO changes
		(tenant_id, client_id, title, description_markdown, risk_level,
		 window_start, window_end, rollback_plan_markdown, validation_plan_markdown, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, tenant_id, client_id, title, description_markdown,
		  risk_level, status, window_start, window_end,
		  rollback_plan_markdown, validation_plan_markdown,
		  created_by, approved_by, approved_at, completed_at,
		  created_at, updated_at`
	var c Change
	err := r.db.QueryRow(ctx, query,
		in.TenantID, in.ClientID, in.Title, in.DescriptionMarkdown, in.RiskLevel,
		in.WindowStart, in.WindowEnd, in.RollbackPlanMarkdown, in.ValidationPlanMarkdown,
		in.CreatedBy,
	).Scan(
		&c.ID, &c.TenantID, &c.ClientID, &c.Title, &c.DescriptionMarkdown,
		&c.RiskLevel, &c.Status, &c.WindowStart, &c.WindowEnd,
		&c.RollbackPlanMarkdown, &c.ValidationPlanMarkdown,
		&c.CreatedBy, &c.ApprovedBy, &c.ApprovedAt, &c.CompletedAt,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create change: %w", err)
	}
	return &c, nil
}

func (r *Repository) Update(ctx context.Context, tenantID, id uuid.UUID, in UpdateInput) (*Change, error) {
	query := `UPDATE changes SET
		title = $3, description_markdown = $4, client_id = $5, risk_level = $6,
		window_start = $7, window_end = $8,
		rollback_plan_markdown = $9, validation_plan_markdown = $10,
		updated_at = NOW()
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, tenant_id, client_id, title, description_markdown,
		  risk_level, status, window_start, window_end,
		  rollback_plan_markdown, validation_plan_markdown,
		  created_by, approved_by, approved_at, completed_at,
		  created_at, updated_at`
	var c Change
	err := r.db.QueryRow(ctx, query,
		tenantID, id, in.Title, in.DescriptionMarkdown, in.ClientID, in.RiskLevel,
		in.WindowStart, in.WindowEnd, in.RollbackPlanMarkdown, in.ValidationPlanMarkdown,
	).Scan(
		&c.ID, &c.TenantID, &c.ClientID, &c.Title, &c.DescriptionMarkdown,
		&c.RiskLevel, &c.Status, &c.WindowStart, &c.WindowEnd,
		&c.RollbackPlanMarkdown, &c.ValidationPlanMarkdown,
		&c.CreatedBy, &c.ApprovedBy, &c.ApprovedAt, &c.CompletedAt,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

// Transition changes the status of a change record.
// Valid transitions: draft→approved, approved→in_progress, in_progress→completed|rolled_back, draft→cancelled, approved→cancelled
func (r *Repository) Transition(ctx context.Context, tenantID, id, actorID uuid.UUID, newStatus string) (*Change, error) {
	current, err := r.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	if !validTransition(current.Status, newStatus) {
		return nil, fmt.Errorf("invalid transition from %s to %s", current.Status, newStatus)
	}

	now := time.Now()
	var approvedBy *uuid.UUID
	var approvedAt, completedAt *time.Time

	switch newStatus {
	case "approved":
		approvedBy = &actorID
		approvedAt = &now
	case "completed", "rolled_back":
		completedAt = &now
	}

	query := `UPDATE changes SET
		status = $3, approved_by = COALESCE($4, approved_by),
		approved_at = COALESCE($5, approved_at),
		completed_at = COALESCE($6, completed_at),
		updated_at = NOW()
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, tenant_id, client_id, title, description_markdown,
		  risk_level, status, window_start, window_end,
		  rollback_plan_markdown, validation_plan_markdown,
		  created_by, approved_by, approved_at, completed_at,
		  created_at, updated_at`
	var c Change
	err = r.db.QueryRow(ctx, query,
		tenantID, id, newStatus, approvedBy, approvedAt, completedAt,
	).Scan(
		&c.ID, &c.TenantID, &c.ClientID, &c.Title, &c.DescriptionMarkdown,
		&c.RiskLevel, &c.Status, &c.WindowStart, &c.WindowEnd,
		&c.RollbackPlanMarkdown, &c.ValidationPlanMarkdown,
		&c.CreatedBy, &c.ApprovedBy, &c.ApprovedAt, &c.CompletedAt,
		&c.CreatedAt, &c.UpdatedAt,
	)
	return &c, err
}

func (r *Repository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	// Only drafts and cancelled can be deleted
	result, err := r.db.Exec(ctx,
		`DELETE FROM changes WHERE tenant_id = $1 AND id = $2 AND status IN ('draft', 'cancelled')`,
		tenantID, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Links ---

func (r *Repository) AddLink(ctx context.Context, tenantID, changeID uuid.UUID, linkedType string, linkedID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO change_links (tenant_id, change_id, linked_type, linked_id)
		 VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
		tenantID, changeID, linkedType, linkedID)
	return err
}

func (r *Repository) RemoveLink(ctx context.Context, tenantID, linkID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM change_links WHERE tenant_id = $1 AND id = $2`,
		tenantID, linkID)
	return err
}

func (r *Repository) ListLinks(ctx context.Context, tenantID, changeID uuid.UUID) ([]ChangeLink, error) {
	rows, err := r.db.Query(ctx, `
		SELECT cl.id, cl.tenant_id, cl.change_id, cl.linked_type, cl.linked_id, cl.created_at,
		       CASE cl.linked_type
		           WHEN 'document' THEN COALESCE((SELECT d.title FROM documents d WHERE d.id = cl.linked_id), 'Unknown')
		           WHEN 'checklist_instance' THEN COALESCE((SELECT ci.status FROM checklist_instances ci WHERE ci.id = cl.linked_id), 'Unknown')
		           WHEN 'evidence_bundle' THEN COALESCE((SELECT eb.name FROM evidence_bundles eb WHERE eb.id = cl.linked_id), 'Unknown')
		       END
		FROM change_links cl
		WHERE cl.tenant_id = $1 AND cl.change_id = $2
		ORDER BY cl.created_at
	`, tenantID, changeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []ChangeLink
	for rows.Next() {
		var l ChangeLink
		if err := rows.Scan(&l.ID, &l.TenantID, &l.ChangeID, &l.LinkedType, &l.LinkedID, &l.CreatedAt, &l.LinkedName); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// --- Helpers ---

func validTransition(from, to string) bool {
	switch from {
	case "draft":
		return to == "approved" || to == "cancelled"
	case "approved":
		return to == "in_progress" || to == "cancelled"
	case "in_progress":
		return to == "completed" || to == "rolled_back"
	}
	return false
}

var ValidStatuses = []string{"draft", "approved", "in_progress", "completed", "rolled_back", "cancelled"}
var ValidRiskLevels = []string{"low", "medium", "high", "critical"}

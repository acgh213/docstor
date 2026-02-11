package sites

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("site not found")

type Site struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	ClientID  uuid.UUID
	Name      string
	Address   string
	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time

	// Joined
	ClientName string
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context, tenantID uuid.UUID, clientID *uuid.UUID) ([]Site, error) {
	query := `
		SELECT s.id, s.tenant_id, s.client_id, s.name, s.address, s.notes, s.created_at, s.updated_at,
		       c.name
		FROM sites s
		LEFT JOIN clients c ON c.id = s.client_id AND c.tenant_id = s.tenant_id
		WHERE s.tenant_id = $1`
	args := []any{tenantID}
	if clientID != nil {
		query += " AND s.client_id = $2"
		args = append(args, *clientID)
	}
	query += " ORDER BY c.name, s.name"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query sites: %w", err)
	}
	defer rows.Close()

	var sites []Site
	for rows.Next() {
		var s Site
		var address, notes *string
		var clientName *string
		if err := rows.Scan(&s.ID, &s.TenantID, &s.ClientID, &s.Name, &address, &notes,
			&s.CreatedAt, &s.UpdatedAt, &clientName); err != nil {
			return nil, fmt.Errorf("scan site: %w", err)
		}
		if address != nil {
			s.Address = *address
		}
		if notes != nil {
			s.Notes = *notes
		}
		if clientName != nil {
			s.ClientName = *clientName
		}
		sites = append(sites, s)
	}
	return sites, nil
}

func (r *Repository) Get(ctx context.Context, tenantID, siteID uuid.UUID) (*Site, error) {
	var s Site
	var address, notes, clientName *string
	err := r.db.QueryRow(ctx, `
		SELECT s.id, s.tenant_id, s.client_id, s.name, s.address, s.notes, s.created_at, s.updated_at,
		       c.name
		FROM sites s
		LEFT JOIN clients c ON c.id = s.client_id AND c.tenant_id = s.tenant_id
		WHERE s.tenant_id = $1 AND s.id = $2
	`, tenantID, siteID).Scan(&s.ID, &s.TenantID, &s.ClientID, &s.Name, &address, &notes,
		&s.CreatedAt, &s.UpdatedAt, &clientName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query site: %w", err)
	}
	if address != nil {
		s.Address = *address
	}
	if notes != nil {
		s.Notes = *notes
	}
	if clientName != nil {
		s.ClientName = *clientName
	}
	return &s, nil
}

type CreateInput struct {
	TenantID uuid.UUID
	ClientID uuid.UUID
	Name     string
	Address  string
	Notes    string
}

func (r *Repository) Create(ctx context.Context, input CreateInput) (*Site, error) {
	var s Site
	var address, notes *string
	if input.Address != "" {
		address = &input.Address
	}
	if input.Notes != "" {
		notes = &input.Notes
	}

	err := r.db.QueryRow(ctx, `
		INSERT INTO sites (tenant_id, client_id, name, address, notes)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, tenant_id, client_id, name, address, notes, created_at, updated_at
	`, input.TenantID, input.ClientID, input.Name, address, notes).Scan(
		&s.ID, &s.TenantID, &s.ClientID, &s.Name, &address, &notes, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert site: %w", err)
	}
	if address != nil {
		s.Address = *address
	}
	if notes != nil {
		s.Notes = *notes
	}
	return &s, nil
}

type UpdateInput struct {
	ClientID uuid.UUID
	Name     string
	Address  string
	Notes    string
}

func (r *Repository) Update(ctx context.Context, tenantID, siteID uuid.UUID, input UpdateInput) (*Site, error) {
	var s Site
	var address, notes *string
	if input.Address != "" {
		address = &input.Address
	}
	if input.Notes != "" {
		notes = &input.Notes
	}

	err := r.db.QueryRow(ctx, `
		UPDATE sites SET client_id = $3, name = $4, address = $5, notes = $6, updated_at = NOW()
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, tenant_id, client_id, name, address, notes, created_at, updated_at
	`, tenantID, siteID, input.ClientID, input.Name, address, notes).Scan(
		&s.ID, &s.TenantID, &s.ClientID, &s.Name, &address, &notes, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update site: %w", err)
	}
	if address != nil {
		s.Address = *address
	}
	if notes != nil {
		s.Notes = *notes
	}
	return &s, nil
}

func (r *Repository) Delete(ctx context.Context, tenantID, siteID uuid.UUID) error {
	result, err := r.db.Exec(ctx, `DELETE FROM sites WHERE tenant_id = $1 AND id = $2`,
		tenantID, siteID)
	if err != nil {
		return fmt.Errorf("delete site: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByClient returns sites for a specific client.
func (r *Repository) ListByClient(ctx context.Context, tenantID, clientID uuid.UUID) ([]Site, error) {
	return r.List(ctx, tenantID, &clientID)
}

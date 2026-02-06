package clients

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("client not found")

type Client struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Name      string
	Code      string
	Notes     string
	CreatedAt time.Time
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context, tenantID uuid.UUID) ([]Client, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, tenant_id, name, code, notes, created_at
		FROM clients
		WHERE tenant_id = $1
		ORDER BY name ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("query clients: %w", err)
	}
	defer rows.Close()

	var clients []Client
	for rows.Next() {
		var c Client
		var notes *string
		if err := rows.Scan(&c.ID, &c.TenantID, &c.Name, &c.Code, &notes, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan client: %w", err)
		}
		if notes != nil {
			c.Notes = *notes
		}
		clients = append(clients, c)
	}

	return clients, nil
}

func (r *Repository) Get(ctx context.Context, tenantID, clientID uuid.UUID) (*Client, error) {
	var c Client
	var notes *string
	err := r.db.QueryRow(ctx, `
		SELECT id, tenant_id, name, code, notes, created_at
		FROM clients
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, clientID).Scan(&c.ID, &c.TenantID, &c.Name, &c.Code, &notes, &c.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query client: %w", err)
	}

	if notes != nil {
		c.Notes = *notes
	}

	return &c, nil
}

type CreateInput struct {
	TenantID uuid.UUID
	Name     string
	Code     string
	Notes    string
}

func (r *Repository) Create(ctx context.Context, input CreateInput) (*Client, error) {
	var c Client
	var notes *string
	if input.Notes != "" {
		notes = &input.Notes
	}

	err := r.db.QueryRow(ctx, `
		INSERT INTO clients (tenant_id, name, code, notes)
		VALUES ($1, $2, $3, $4)
		RETURNING id, tenant_id, name, code, notes, created_at
	`, input.TenantID, input.Name, input.Code, notes).Scan(&c.ID, &c.TenantID, &c.Name, &c.Code, &notes, &c.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("insert client: %w", err)
	}

	if notes != nil {
		c.Notes = *notes
	}

	return &c, nil
}

type UpdateInput struct {
	Name  string
	Code  string
	Notes string
}

func (r *Repository) Update(ctx context.Context, tenantID, clientID uuid.UUID, input UpdateInput) (*Client, error) {
	var c Client
	var notes *string
	if input.Notes != "" {
		notes = &input.Notes
	}

	err := r.db.QueryRow(ctx, `
		UPDATE clients
		SET name = $3, code = $4, notes = $5
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, tenant_id, name, code, notes, created_at
	`, tenantID, clientID, input.Name, input.Code, notes).Scan(&c.ID, &c.TenantID, &c.Name, &c.Code, &notes, &c.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update client: %w", err)
	}

	if notes != nil {
		c.Notes = *notes
	}

	return &c, nil
}

package templates

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("template not found")

type Template struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	Name             string
	TemplateType     string // "doc" or "runbook"
	BodyMarkdown     string
	DefaultMetadata  map[string]any
	CreatedBy        uuid.UUID
	CreatedAt        time.Time
	UpdatedAt        time.Time

	// Joined
	Author *UserInfo
}

type UserInfo struct {
	ID    uuid.UUID
	Name  string
	Email string
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context, tenantID uuid.UUID) ([]Template, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.tenant_id, t.name, t.template_type, t.body_markdown, t.default_metadata_json,
		       t.created_by, t.created_at, t.updated_at,
		       u.id, u.name, u.email
		FROM templates t
		JOIN users u ON t.created_by = u.id
		WHERE t.tenant_id = $1
		ORDER BY t.name ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("query templates: %w", err)
	}
	defer rows.Close()

	var templates []Template
	for rows.Next() {
		var t Template
		var author UserInfo
		if err := rows.Scan(
			&t.ID, &t.TenantID, &t.Name, &t.TemplateType, &t.BodyMarkdown, &t.DefaultMetadata,
			&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt,
			&author.ID, &author.Name, &author.Email,
		); err != nil {
			return nil, fmt.Errorf("scan template: %w", err)
		}
		t.Author = &author
		templates = append(templates, t)
	}
	return templates, nil
}

func (r *Repository) Get(ctx context.Context, tenantID, templateID uuid.UUID) (*Template, error) {
	var t Template
	var author UserInfo
	err := r.db.QueryRow(ctx, `
		SELECT t.id, t.tenant_id, t.name, t.template_type, t.body_markdown, t.default_metadata_json,
		       t.created_by, t.created_at, t.updated_at,
		       u.id, u.name, u.email
		FROM templates t
		JOIN users u ON t.created_by = u.id
		WHERE t.tenant_id = $1 AND t.id = $2
	`, tenantID, templateID).Scan(
		&t.ID, &t.TenantID, &t.Name, &t.TemplateType, &t.BodyMarkdown, &t.DefaultMetadata,
		&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt,
		&author.ID, &author.Name, &author.Email,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query template: %w", err)
	}
	t.Author = &author
	return &t, nil
}

type CreateInput struct {
	TenantID     uuid.UUID
	Name         string
	TemplateType string
	BodyMarkdown string
	CreatedBy    uuid.UUID
}

func (r *Repository) Create(ctx context.Context, input CreateInput) (*Template, error) {
	if input.TemplateType == "" {
		input.TemplateType = "doc"
	}

	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO templates (tenant_id, name, template_type, body_markdown, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, input.TenantID, input.Name, input.TemplateType, input.BodyMarkdown, input.CreatedBy).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("insert template: %w", err)
	}
	return r.Get(ctx, input.TenantID, id)
}

type UpdateInput struct {
	Name         string
	TemplateType string
	BodyMarkdown string
}

func (r *Repository) Update(ctx context.Context, tenantID, templateID uuid.UUID, input UpdateInput) (*Template, error) {
	result, err := r.db.Exec(ctx, `
		UPDATE templates
		SET name = $3, template_type = $4, body_markdown = $5, updated_at = NOW()
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, templateID, input.Name, input.TemplateType, input.BodyMarkdown)
	if err != nil {
		return nil, fmt.Errorf("update template: %w", err)
	}
	if result.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return r.Get(ctx, tenantID, templateID)
}

func (r *Repository) Delete(ctx context.Context, tenantID, templateID uuid.UUID) error {
	result, err := r.db.Exec(ctx, `DELETE FROM templates WHERE tenant_id = $1 AND id = $2`,
		tenantID, templateID)
	if err != nil {
		return fmt.Errorf("delete template: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

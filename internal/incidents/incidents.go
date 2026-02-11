package incidents

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

func ns(p *string) string {
	if p != nil {
		return *p
	}
	return ""
}

// ===================== KNOWN ISSUES =====================

type KnownIssue struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	ClientID          *uuid.UUID
	Title             string
	Severity          string
	Status            string
	Description       string
	Workaround        string
	LinkedDocumentID  *uuid.UUID
	CreatedBy         *uuid.UUID
	CreatedAt         time.Time
	UpdatedAt         time.Time
	// joined fields
	ClientName        string
	CreatedByName     string
}

type Incident struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	ClientID  *uuid.UUID
	Title     string
	Severity  string
	Status    string
	StartedAt time.Time
	EndedAt   *time.Time
	Summary   string
	CreatedBy *uuid.UUID
	CreatedAt time.Time
	// joined
	ClientName    string
	CreatedByName string
	EventCount    int
}

type IncidentEvent struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	IncidentID  uuid.UUID
	EventType   string
	Detail      string
	ActorUserID *uuid.UUID
	CreatedAt   time.Time
	// joined
	ActorName string
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// --- Known Issues ---

func (r *Repository) ListKnownIssues(ctx context.Context, tenantID uuid.UUID, status string, clientID *uuid.UUID) ([]KnownIssue, error) {
	query := `SELECT ki.id, ki.tenant_id, ki.client_id, ki.title, ki.severity, ki.status, ki.description, ki.workaround, ki.linked_document_id, ki.created_by, ki.created_at, ki.updated_at,
		COALESCE(c.name,'') as client_name, COALESCE(u.name,'') as created_by_name
		FROM known_issues ki
		LEFT JOIN clients c ON c.id = ki.client_id
		LEFT JOIN users u ON u.id = ki.created_by
		WHERE ki.tenant_id = $1`
	args := []any{tenantID}
	n := 2
	if status != "" {
		query += fmt.Sprintf(" AND ki.status = $%d", n)
		args = append(args, status)
		n++
	}
	if clientID != nil {
		query += fmt.Sprintf(" AND ki.client_id = $%d", n)
		args = append(args, *clientID)
		n++
	}
	query += " ORDER BY ki.updated_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query known_issues: %w", err)
	}
	defer rows.Close()

	var out []KnownIssue
	for rows.Next() {
		var ki KnownIssue
		var desc, workaround *string
		if err := rows.Scan(&ki.ID, &ki.TenantID, &ki.ClientID, &ki.Title, &ki.Severity, &ki.Status, &desc, &workaround, &ki.LinkedDocumentID, &ki.CreatedBy, &ki.CreatedAt, &ki.UpdatedAt, &ki.ClientName, &ki.CreatedByName); err != nil {
			return nil, fmt.Errorf("scan known_issue: %w", err)
		}
		ki.Description = ns(desc)
		ki.Workaround = ns(workaround)
		out = append(out, ki)
	}
	return out, nil
}

func (r *Repository) GetKnownIssue(ctx context.Context, tenantID, id uuid.UUID) (*KnownIssue, error) {
	var ki KnownIssue
	var desc, workaround *string
	err := r.db.QueryRow(ctx, `SELECT ki.id, ki.tenant_id, ki.client_id, ki.title, ki.severity, ki.status, ki.description, ki.workaround, ki.linked_document_id, ki.created_by, ki.created_at, ki.updated_at,
		COALESCE(c.name,'') as client_name, COALESCE(u.name,'') as created_by_name
		FROM known_issues ki
		LEFT JOIN clients c ON c.id = ki.client_id
		LEFT JOIN users u ON u.id = ki.created_by
		WHERE ki.tenant_id = $1 AND ki.id = $2`, tenantID, id).Scan(&ki.ID, &ki.TenantID, &ki.ClientID, &ki.Title, &ki.Severity, &ki.Status, &desc, &workaround, &ki.LinkedDocumentID, &ki.CreatedBy, &ki.CreatedAt, &ki.UpdatedAt, &ki.ClientName, &ki.CreatedByName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get known_issue: %w", err)
	}
	ki.Description = ns(desc)
	ki.Workaround = ns(workaround)
	return &ki, nil
}

type CreateKnownIssueInput struct {
	TenantID         uuid.UUID
	ClientID         *uuid.UUID
	Title            string
	Severity         string
	Status           string
	Description      string
	Workaround       string
	LinkedDocumentID *uuid.UUID
	CreatedBy        uuid.UUID
}

func (r *Repository) CreateKnownIssue(ctx context.Context, in CreateKnownIssueInput) (*KnownIssue, error) {
	var ki KnownIssue
	var desc, workaround *string
	if in.Description != "" { desc = &in.Description }
	if in.Workaround != "" { workaround = &in.Workaround }
	err := r.db.QueryRow(ctx, `INSERT INTO known_issues (tenant_id, client_id, title, severity, status, description, workaround, linked_document_id, created_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id, tenant_id, client_id, title, severity, status, description, workaround, linked_document_id, created_by, created_at, updated_at`,
		in.TenantID, in.ClientID, in.Title, in.Severity, in.Status, desc, workaround, in.LinkedDocumentID, in.CreatedBy,
	).Scan(&ki.ID, &ki.TenantID, &ki.ClientID, &ki.Title, &ki.Severity, &ki.Status, &desc, &workaround, &ki.LinkedDocumentID, &ki.CreatedBy, &ki.CreatedAt, &ki.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create known_issue: %w", err)
	}
	ki.Description = ns(desc)
	ki.Workaround = ns(workaround)
	return &ki, nil
}

type UpdateKnownIssueInput struct {
	ClientID         *uuid.UUID
	Title            string
	Severity         string
	Status           string
	Description      string
	Workaround       string
	LinkedDocumentID *uuid.UUID
}

func (r *Repository) UpdateKnownIssue(ctx context.Context, tenantID, id uuid.UUID, in UpdateKnownIssueInput) (*KnownIssue, error) {
	var ki KnownIssue
	var desc, workaround *string
	if in.Description != "" { desc = &in.Description }
	if in.Workaround != "" { workaround = &in.Workaround }
	err := r.db.QueryRow(ctx, `UPDATE known_issues SET client_id=$3, title=$4, severity=$5, status=$6, description=$7, workaround=$8, linked_document_id=$9, updated_at=now() WHERE tenant_id=$1 AND id=$2 RETURNING id, tenant_id, client_id, title, severity, status, description, workaround, linked_document_id, created_by, created_at, updated_at`,
		tenantID, id, in.ClientID, in.Title, in.Severity, in.Status, desc, workaround, in.LinkedDocumentID,
	).Scan(&ki.ID, &ki.TenantID, &ki.ClientID, &ki.Title, &ki.Severity, &ki.Status, &desc, &workaround, &ki.LinkedDocumentID, &ki.CreatedBy, &ki.CreatedAt, &ki.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update known_issue: %w", err)
	}
	ki.Description = ns(desc)
	ki.Workaround = ns(workaround)
	return &ki, nil
}

func (r *Repository) DeleteKnownIssue(ctx context.Context, tenantID, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM known_issues WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return fmt.Errorf("delete known_issue: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Incidents ---

func (r *Repository) ListIncidents(ctx context.Context, tenantID uuid.UUID, status string, clientID *uuid.UUID) ([]Incident, error) {
	query := `SELECT i.id, i.tenant_id, i.client_id, i.title, i.severity, i.status, i.started_at, i.ended_at, i.summary, i.created_by, i.created_at,
		COALESCE(c.name,'') as client_name, COALESCE(u.name,'') as created_by_name,
		(SELECT count(*) FROM incident_events ie WHERE ie.incident_id = i.id) as event_count
		FROM incidents i
		LEFT JOIN clients c ON c.id = i.client_id
		LEFT JOIN users u ON u.id = i.created_by
		WHERE i.tenant_id = $1`
	args := []any{tenantID}
	n := 2
	if status != "" {
		query += fmt.Sprintf(" AND i.status = $%d", n)
		args = append(args, status)
		n++
	}
	if clientID != nil {
		query += fmt.Sprintf(" AND i.client_id = $%d", n)
		args = append(args, *clientID)
		n++
	}
	query += " ORDER BY i.started_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query incidents: %w", err)
	}
	defer rows.Close()

	var out []Incident
	for rows.Next() {
		var inc Incident
		var summary *string
		if err := rows.Scan(&inc.ID, &inc.TenantID, &inc.ClientID, &inc.Title, &inc.Severity, &inc.Status, &inc.StartedAt, &inc.EndedAt, &summary, &inc.CreatedBy, &inc.CreatedAt, &inc.ClientName, &inc.CreatedByName, &inc.EventCount); err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
		}
		inc.Summary = ns(summary)
		out = append(out, inc)
	}
	return out, nil
}

func (r *Repository) GetIncident(ctx context.Context, tenantID, id uuid.UUID) (*Incident, error) {
	var inc Incident
	var summary *string
	err := r.db.QueryRow(ctx, `SELECT i.id, i.tenant_id, i.client_id, i.title, i.severity, i.status, i.started_at, i.ended_at, i.summary, i.created_by, i.created_at,
		COALESCE(c.name,'') as client_name, COALESCE(u.name,'') as created_by_name,
		(SELECT count(*) FROM incident_events ie WHERE ie.incident_id = i.id) as event_count
		FROM incidents i
		LEFT JOIN clients c ON c.id = i.client_id
		LEFT JOIN users u ON u.id = i.created_by
		WHERE i.tenant_id = $1 AND i.id = $2`, tenantID, id).Scan(&inc.ID, &inc.TenantID, &inc.ClientID, &inc.Title, &inc.Severity, &inc.Status, &inc.StartedAt, &inc.EndedAt, &summary, &inc.CreatedBy, &inc.CreatedAt, &inc.ClientName, &inc.CreatedByName, &inc.EventCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get incident: %w", err)
	}
	inc.Summary = ns(summary)
	return &inc, nil
}

type CreateIncidentInput struct {
	TenantID  uuid.UUID
	ClientID  *uuid.UUID
	Title     string
	Severity  string
	Status    string
	StartedAt time.Time
	Summary   string
	CreatedBy uuid.UUID
}

func (r *Repository) CreateIncident(ctx context.Context, in CreateIncidentInput) (*Incident, error) {
	var inc Incident
	var summary *string
	if in.Summary != "" { summary = &in.Summary }
	err := r.db.QueryRow(ctx, `INSERT INTO incidents (tenant_id, client_id, title, severity, status, started_at, summary, created_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id, tenant_id, client_id, title, severity, status, started_at, ended_at, summary, created_by, created_at`,
		in.TenantID, in.ClientID, in.Title, in.Severity, in.Status, in.StartedAt, summary, in.CreatedBy,
	).Scan(&inc.ID, &inc.TenantID, &inc.ClientID, &inc.Title, &inc.Severity, &inc.Status, &inc.StartedAt, &inc.EndedAt, &summary, &inc.CreatedBy, &inc.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create incident: %w", err)
	}
	inc.Summary = ns(summary)
	return &inc, nil
}

type UpdateIncidentInput struct {
	ClientID  *uuid.UUID
	Title     string
	Severity  string
	Status    string
	StartedAt time.Time
	EndedAt   *time.Time
	Summary   string
}

func (r *Repository) UpdateIncident(ctx context.Context, tenantID, id uuid.UUID, in UpdateIncidentInput) (*Incident, error) {
	var inc Incident
	var summary *string
	if in.Summary != "" { summary = &in.Summary }
	err := r.db.QueryRow(ctx, `UPDATE incidents SET client_id=$3, title=$4, severity=$5, status=$6, started_at=$7, ended_at=$8, summary=$9 WHERE tenant_id=$1 AND id=$2 RETURNING id, tenant_id, client_id, title, severity, status, started_at, ended_at, summary, created_by, created_at`,
		tenantID, id, in.ClientID, in.Title, in.Severity, in.Status, in.StartedAt, in.EndedAt, summary,
	).Scan(&inc.ID, &inc.TenantID, &inc.ClientID, &inc.Title, &inc.Severity, &inc.Status, &inc.StartedAt, &inc.EndedAt, &summary, &inc.CreatedBy, &inc.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update incident: %w", err)
	}
	inc.Summary = ns(summary)
	return &inc, nil
}

func (r *Repository) DeleteIncident(ctx context.Context, tenantID, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM incidents WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return fmt.Errorf("delete incident: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Incident Events ---

func (r *Repository) ListEvents(ctx context.Context, tenantID, incidentID uuid.UUID) ([]IncidentEvent, error) {
	rows, err := r.db.Query(ctx, `SELECT ie.id, ie.tenant_id, ie.incident_id, ie.event_type, ie.detail, ie.actor_user_id, ie.created_at,
		COALESCE(u.name,'') as actor_name
		FROM incident_events ie
		LEFT JOIN users u ON u.id = ie.actor_user_id
		WHERE ie.tenant_id = $1 AND ie.incident_id = $2
		ORDER BY ie.created_at ASC`, tenantID, incidentID)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var out []IncidentEvent
	for rows.Next() {
		var e IncidentEvent
		if err := rows.Scan(&e.ID, &e.TenantID, &e.IncidentID, &e.EventType, &e.Detail, &e.ActorUserID, &e.CreatedAt, &e.ActorName); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		out = append(out, e)
	}
	return out, nil
}

type CreateEventInput struct {
	TenantID    uuid.UUID
	IncidentID  uuid.UUID
	EventType   string
	Detail      string
	ActorUserID uuid.UUID
}

func (r *Repository) CreateEvent(ctx context.Context, in CreateEventInput) (*IncidentEvent, error) {
	var e IncidentEvent
	err := r.db.QueryRow(ctx, `INSERT INTO incident_events (tenant_id, incident_id, event_type, detail, actor_user_id) VALUES ($1,$2,$3,$4,$5) RETURNING id, tenant_id, incident_id, event_type, detail, actor_user_id, created_at`,
		in.TenantID, in.IncidentID, in.EventType, in.Detail, in.ActorUserID,
	).Scan(&e.ID, &e.TenantID, &e.IncidentID, &e.EventType, &e.Detail, &e.ActorUserID, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create event: %w", err)
	}
	return &e, nil
}

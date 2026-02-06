package docs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound     = errors.New("document not found")
	ErrPathConflict = errors.New("path already exists")
	ErrConflict     = errors.New("document has been modified")
)

type DocType string

const (
	DocTypeDoc     DocType = "doc"
	DocTypeRunbook DocType = "runbook"
)

type Sensitivity string

const (
	SensitivityPublic       Sensitivity = "public-internal"
	SensitivityRestricted   Sensitivity = "restricted"
	SensitivityConfidential Sensitivity = "confidential"
)

type Document struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	ClientID          *uuid.UUID
	Path              string
	Title             string
	DocType           DocType
	Sensitivity       Sensitivity
	OwnerUserID       *uuid.UUID
	MetadataJSON      map[string]any
	CurrentRevisionID *uuid.UUID
	CreatedBy         uuid.UUID
	CreatedAt         time.Time
	UpdatedAt         time.Time

	// Joined fields
	CurrentRevision *Revision
	Client          *ClientInfo
	Owner           *UserInfo
}

type Revision struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	DocumentID     uuid.UUID
	BodyMarkdown   string
	CreatedBy      uuid.UUID
	CreatedAt      time.Time
	Message        string
	BaseRevisionID *uuid.UUID

	// Joined fields
	Author *UserInfo
}

type ClientInfo struct {
	ID   uuid.UUID
	Name string
	Code string
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

func (r *Repository) List(ctx context.Context, tenantID uuid.UUID, clientID *uuid.UUID, docType *DocType) ([]Document, error) {
	query := `
		SELECT d.id, d.tenant_id, d.client_id, d.path, d.title, d.doc_type, d.sensitivity,
		       d.owner_user_id, d.metadata_json, d.current_revision_id, d.created_by, d.created_at, d.updated_at,
		       c.id, c.name, c.code,
		       u.id, u.name, u.email
		FROM documents d
		LEFT JOIN clients c ON d.client_id = c.id
		LEFT JOIN users u ON d.owner_user_id = u.id
		WHERE d.tenant_id = $1
	`
	args := []any{tenantID}
	argIdx := 2

	if clientID != nil {
		query += fmt.Sprintf(" AND d.client_id = $%d", argIdx)
		args = append(args, *clientID)
		argIdx++
	}

	if docType != nil {
		query += fmt.Sprintf(" AND d.doc_type = $%d", argIdx)
		args = append(args, *docType)
	}

	query += " ORDER BY d.updated_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query documents: %w", err)
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		var d Document
		var clientID, clientName, clientCode *string
		var ownerID, ownerName, ownerEmail *string

		err := rows.Scan(
			&d.ID, &d.TenantID, &d.ClientID, &d.Path, &d.Title, &d.DocType, &d.Sensitivity,
			&d.OwnerUserID, &d.MetadataJSON, &d.CurrentRevisionID, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt,
			&clientID, &clientName, &clientCode,
			&ownerID, &ownerName, &ownerEmail,
		)
		if err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}

		if clientID != nil {
			cid, _ := uuid.Parse(*clientID)
			d.Client = &ClientInfo{ID: cid, Name: *clientName, Code: *clientCode}
		}
		if ownerID != nil {
			oid, _ := uuid.Parse(*ownerID)
			d.Owner = &UserInfo{ID: oid, Name: *ownerName, Email: *ownerEmail}
		}

		docs = append(docs, d)
	}

	return docs, nil
}

func (r *Repository) GetByPath(ctx context.Context, tenantID uuid.UUID, path string) (*Document, error) {
	path = normalizePath(path)

	var d Document
	var clientID *uuid.UUID
	var ownerID *uuid.UUID

	err := r.db.QueryRow(ctx, `
		SELECT id, tenant_id, client_id, path, title, doc_type, sensitivity,
		       owner_user_id, metadata_json, current_revision_id, created_by, created_at, updated_at
		FROM documents
		WHERE tenant_id = $1 AND path = $2
	`, tenantID, path).Scan(
		&d.ID, &d.TenantID, &clientID, &d.Path, &d.Title, &d.DocType, &d.Sensitivity,
		&ownerID, &d.MetadataJSON, &d.CurrentRevisionID, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query document: %w", err)
	}

	d.ClientID = clientID
	d.OwnerUserID = ownerID

	// Load current revision
	if d.CurrentRevisionID != nil {
		rev, err := r.GetRevision(ctx, tenantID, *d.CurrentRevisionID)
		if err == nil {
			d.CurrentRevision = rev
		}
	}

	// Load client info
	if d.ClientID != nil {
		var ci ClientInfo
		err := r.db.QueryRow(ctx, `SELECT id, name, code FROM clients WHERE id = $1`, *d.ClientID).Scan(&ci.ID, &ci.Name, &ci.Code)
		if err == nil {
			d.Client = &ci
		}
	}

	// Load owner info
	if d.OwnerUserID != nil {
		var ui UserInfo
		err := r.db.QueryRow(ctx, `SELECT id, name, email FROM users WHERE id = $1`, *d.OwnerUserID).Scan(&ui.ID, &ui.Name, &ui.Email)
		if err == nil {
			d.Owner = &ui
		}
	}

	return &d, nil
}

func (r *Repository) GetByID(ctx context.Context, tenantID, docID uuid.UUID) (*Document, error) {
	var d Document
	var clientID *uuid.UUID
	var ownerID *uuid.UUID

	err := r.db.QueryRow(ctx, `
		SELECT id, tenant_id, client_id, path, title, doc_type, sensitivity,
		       owner_user_id, metadata_json, current_revision_id, created_by, created_at, updated_at
		FROM documents
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, docID).Scan(
		&d.ID, &d.TenantID, &clientID, &d.Path, &d.Title, &d.DocType, &d.Sensitivity,
		&ownerID, &d.MetadataJSON, &d.CurrentRevisionID, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query document: %w", err)
	}

	d.ClientID = clientID
	d.OwnerUserID = ownerID

	// Load current revision
	if d.CurrentRevisionID != nil {
		rev, err := r.GetRevision(ctx, tenantID, *d.CurrentRevisionID)
		if err == nil {
			d.CurrentRevision = rev
		}
	}

	return &d, nil
}

type CreateInput struct {
	TenantID    uuid.UUID
	ClientID    *uuid.UUID
	Path        string
	Title       string
	DocType     DocType
	Sensitivity Sensitivity
	OwnerUserID *uuid.UUID
	CreatedBy   uuid.UUID
	Body        string
	Message     string
}

func (r *Repository) Create(ctx context.Context, input CreateInput) (*Document, error) {
	input.Path = normalizePath(input.Path)

	if input.DocType == "" {
		input.DocType = DocTypeDoc
	}
	if input.Sensitivity == "" {
		input.Sensitivity = SensitivityPublic
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Create document
	var docID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO documents (tenant_id, client_id, path, title, doc_type, sensitivity, owner_user_id, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, input.TenantID, input.ClientID, input.Path, input.Title, input.DocType, input.Sensitivity, input.OwnerUserID, input.CreatedBy).Scan(&docID)
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			return nil, ErrPathConflict
		}
		return nil, fmt.Errorf("insert document: %w", err)
	}

	// Create initial revision
	var revID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO revisions (tenant_id, document_id, body_markdown, created_by, message)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, input.TenantID, docID, input.Body, input.CreatedBy, input.Message).Scan(&revID)
	if err != nil {
		return nil, fmt.Errorf("insert revision: %w", err)
	}

	// Update document with current revision
	_, err = tx.Exec(ctx, `UPDATE documents SET current_revision_id = $1 WHERE id = $2`, revID, docID)
	if err != nil {
		return nil, fmt.Errorf("update current revision: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return r.GetByID(ctx, input.TenantID, docID)
}

type UpdateInput struct {
	Body           string
	Message        string
	BaseRevisionID uuid.UUID
	UpdatedBy      uuid.UUID
}

func (r *Repository) Update(ctx context.Context, tenantID, docID uuid.UUID, input UpdateInput) (*Document, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Check for conflict
	var currentRevID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT current_revision_id FROM documents WHERE tenant_id = $1 AND id = $2 FOR UPDATE`,
		tenantID, docID).Scan(&currentRevID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query document: %w", err)
	}

	if currentRevID != input.BaseRevisionID {
		return nil, ErrConflict
	}

	// Create new revision
	var revID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO revisions (tenant_id, document_id, body_markdown, created_by, message, base_revision_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, tenantID, docID, input.Body, input.UpdatedBy, input.Message, input.BaseRevisionID).Scan(&revID)
	if err != nil {
		return nil, fmt.Errorf("insert revision: %w", err)
	}

	// Update document
	_, err = tx.Exec(ctx, `UPDATE documents SET current_revision_id = $1, updated_at = NOW() WHERE id = $2`, revID, docID)
	if err != nil {
		return nil, fmt.Errorf("update document: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return r.GetByID(ctx, tenantID, docID)
}

func (r *Repository) GetRevision(ctx context.Context, tenantID, revID uuid.UUID) (*Revision, error) {
	var rev Revision
	var baseRevID *uuid.UUID
	var message *string
	var author UserInfo

	err := r.db.QueryRow(ctx, `
		SELECT r.id, r.tenant_id, r.document_id, r.body_markdown, r.created_by, r.created_at, r.message, r.base_revision_id,
		       u.id, u.name, u.email
		FROM revisions r
		JOIN users u ON r.created_by = u.id
		WHERE r.tenant_id = $1 AND r.id = $2
	`, tenantID, revID).Scan(
		&rev.ID, &rev.TenantID, &rev.DocumentID, &rev.BodyMarkdown, &rev.CreatedBy, &rev.CreatedAt, &message, &baseRevID,
		&author.ID, &author.Name, &author.Email,
	)

	if err == nil {
		rev.Author = &author
	} else if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else {
		// Join might fail if user was deleted - try without
		err = r.db.QueryRow(ctx, `
			SELECT id, tenant_id, document_id, body_markdown, created_by, created_at, message, base_revision_id
			FROM revisions
			WHERE tenant_id = $1 AND id = $2
		`, tenantID, revID).Scan(
			&rev.ID, &rev.TenantID, &rev.DocumentID, &rev.BodyMarkdown, &rev.CreatedBy, &rev.CreatedAt, &message, &baseRevID,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("query revision: %w", err)
		}
	}

	if message != nil {
		rev.Message = *message
	}
	rev.BaseRevisionID = baseRevID

	return &rev, nil
}

func (r *Repository) ListRevisions(ctx context.Context, tenantID, docID uuid.UUID) ([]Revision, error) {
	rows, err := r.db.Query(ctx, `
		SELECT r.id, r.tenant_id, r.document_id, r.body_markdown, r.created_by, r.created_at, r.message, r.base_revision_id,
		       u.id, u.name, u.email
		FROM revisions r
		JOIN users u ON r.created_by = u.id
		WHERE r.tenant_id = $1 AND r.document_id = $2
		ORDER BY r.created_at DESC
	`, tenantID, docID)
	if err != nil {
		return nil, fmt.Errorf("query revisions: %w", err)
	}
	defer rows.Close()

	var revisions []Revision
	for rows.Next() {
		var rev Revision
		var message *string
		var baseRevID *uuid.UUID
		var author UserInfo

		err := rows.Scan(
			&rev.ID, &rev.TenantID, &rev.DocumentID, &rev.BodyMarkdown, &rev.CreatedBy, &rev.CreatedAt, &message, &baseRevID,
			&author.ID, &author.Name, &author.Email,
		)
		if err != nil {
			return nil, fmt.Errorf("scan revision: %w", err)
		}

		if message != nil {
			rev.Message = *message
		}
		rev.BaseRevisionID = baseRevID
		rev.Author = &author

		revisions = append(revisions, rev)
	}

	return revisions, nil
}

func normalizePath(path string) string {
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")
	return strings.ToLower(path)
}

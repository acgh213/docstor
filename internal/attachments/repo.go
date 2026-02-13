package attachments

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Attachment represents a stored file
type Attachment struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Filename    string
	ContentType string
	SizeBytes   int64
	SHA256      string
	StorageKey  string
	CreatedBy   uuid.UUID
	CreatedAt   time.Time
}

// AttachmentLink represents a link between an attachment and another entity
type AttachmentLink struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	AttachmentID uuid.UUID
	LinkedType   string // 'document', 'revision', 'incident', 'change'
	LinkedID     uuid.UUID
	CreatedAt    time.Time
}

// EvidenceBundle represents a collection of attachments for export
type EvidenceBundle struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Name        string
	Description string
	CreatedBy   uuid.UUID
	CreatedAt   time.Time
	ItemCount   int // populated by queries
}

// EvidenceBundleItem represents an attachment in a bundle
type EvidenceBundleItem struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	BundleID     uuid.UUID
	AttachmentID uuid.UUID
	Note         string
	CreatedAt    time.Time
	Attachment   *Attachment // populated by queries
}

// Repo handles attachment database operations
type Repo struct {
	db *pgxpool.Pool
}

// NewRepo creates a new attachments repository
func NewRepo(db *pgxpool.Pool) *Repo {
	return &Repo{db: db}
}

// CreateAttachment inserts a new attachment record
func (r *Repo) CreateAttachment(ctx context.Context, a *Attachment) error {
	query := `
		INSERT INTO attachments (tenant_id, filename, content_type, size_bytes, sha256, storage_key, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`
	return r.db.QueryRow(ctx, query,
		a.TenantID, a.Filename, a.ContentType, a.SizeBytes, a.SHA256, a.StorageKey, a.CreatedBy,
	).Scan(&a.ID, &a.CreatedAt)
}

// GetAttachment retrieves an attachment by ID (tenant-scoped)
func (r *Repo) GetAttachment(ctx context.Context, tenantID, id uuid.UUID) (*Attachment, error) {
	query := `
		SELECT id, tenant_id, filename, content_type, size_bytes, sha256, storage_key, created_by, created_at
		FROM attachments
		WHERE tenant_id = $1 AND id = $2`
	a := &Attachment{}
	err := r.db.QueryRow(ctx, query, tenantID, id).Scan(
		&a.ID, &a.TenantID, &a.Filename, &a.ContentType, &a.SizeBytes, &a.SHA256, &a.StorageKey, &a.CreatedBy, &a.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get attachment: %w", err)
	}
	return a, nil
}

// FindBySHA256 finds an existing attachment by hash (for deduplication)
func (r *Repo) FindBySHA256(ctx context.Context, tenantID uuid.UUID, sha256 string) (*Attachment, error) {
	query := `
		SELECT id, tenant_id, filename, content_type, size_bytes, sha256, storage_key, created_by, created_at
		FROM attachments
		WHERE tenant_id = $1 AND sha256 = $2
		LIMIT 1`
	a := &Attachment{}
	err := r.db.QueryRow(ctx, query, tenantID, sha256).Scan(
		&a.ID, &a.TenantID, &a.Filename, &a.ContentType, &a.SizeBytes, &a.SHA256, &a.StorageKey, &a.CreatedBy, &a.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find by sha256: %w", err)
	}
	return a, nil
}

// ListByDocument lists all attachments linked to a document
func (r *Repo) ListByDocument(ctx context.Context, tenantID, documentID uuid.UUID) ([]*Attachment, error) {
	query := `
		SELECT a.id, a.tenant_id, a.filename, a.content_type, a.size_bytes, a.sha256, a.storage_key, a.created_by, a.created_at
		FROM attachments a
		JOIN attachment_links l ON l.attachment_id = a.id AND l.tenant_id = a.tenant_id
		WHERE a.tenant_id = $1 AND l.linked_type = 'document' AND l.linked_id = $2
		ORDER BY a.created_at DESC`
	return r.queryAttachments(ctx, query, tenantID, documentID)
}

// ListAll lists all attachments for a tenant (most recent first)
func (r *Repo) ListAll(ctx context.Context, tenantID uuid.UUID) ([]*Attachment, error) {
	return r.queryAttachments(ctx, `
		SELECT id, tenant_id, filename, content_type, size_bytes, sha256, storage_key, created_by, created_at
		FROM attachments WHERE tenant_id = $1
		ORDER BY created_at DESC LIMIT 200`, tenantID)
}

func (r *Repo) queryAttachments(ctx context.Context, query string, args ...any) ([]*Attachment, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query attachments: %w", err)
	}
	defer rows.Close()

	var attachments []*Attachment
	for rows.Next() {
		a := &Attachment{}
		if err := rows.Scan(
			&a.ID, &a.TenantID, &a.Filename, &a.ContentType, &a.SizeBytes, &a.SHA256, &a.StorageKey, &a.CreatedBy, &a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		attachments = append(attachments, a)
	}
	return attachments, rows.Err()
}

// CreateLink creates a link between an attachment and another entity
func (r *Repo) CreateLink(ctx context.Context, link *AttachmentLink) error {
	query := `
		INSERT INTO attachment_links (tenant_id, attachment_id, linked_type, linked_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`
	return r.db.QueryRow(ctx, query,
		link.TenantID, link.AttachmentID, link.LinkedType, link.LinkedID,
	).Scan(&link.ID, &link.CreatedAt)
}

// DeleteLink removes a link
func (r *Repo) DeleteLink(ctx context.Context, tenantID, linkID uuid.UUID) error {
	query := `DELETE FROM attachment_links WHERE tenant_id = $1 AND id = $2`
	_, err := r.db.Exec(ctx, query, tenantID, linkID)
	return err
}

// GetLinkByAttachmentAndDoc gets a specific link
func (r *Repo) GetLinkByAttachmentAndDoc(ctx context.Context, tenantID, attachmentID, documentID uuid.UUID) (*AttachmentLink, error) {
	query := `
		SELECT id, tenant_id, attachment_id, linked_type, linked_id, created_at
		FROM attachment_links
		WHERE tenant_id = $1 AND attachment_id = $2 AND linked_type = 'document' AND linked_id = $3`
	link := &AttachmentLink{}
	err := r.db.QueryRow(ctx, query, tenantID, attachmentID, documentID).Scan(
		&link.ID, &link.TenantID, &link.AttachmentID, &link.LinkedType, &link.LinkedID, &link.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return link, nil
}

// --- Evidence Bundles ---

// CreateBundle creates a new evidence bundle
func (r *Repo) CreateBundle(ctx context.Context, b *EvidenceBundle) error {
	query := `
		INSERT INTO evidence_bundles (tenant_id, name, description, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`
	return r.db.QueryRow(ctx, query,
		b.TenantID, b.Name, b.Description, b.CreatedBy,
	).Scan(&b.ID, &b.CreatedAt)
}

// GetBundle retrieves a bundle by ID
func (r *Repo) GetBundle(ctx context.Context, tenantID, id uuid.UUID) (*EvidenceBundle, error) {
	query := `
		SELECT b.id, b.tenant_id, b.name, b.description, b.created_by, b.created_at,
		       (SELECT COUNT(*) FROM evidence_bundle_items WHERE bundle_id = b.id)
		FROM evidence_bundles b
		WHERE b.tenant_id = $1 AND b.id = $2`
	b := &EvidenceBundle{}
	var desc *string
	err := r.db.QueryRow(ctx, query, tenantID, id).Scan(
		&b.ID, &b.TenantID, &b.Name, &desc, &b.CreatedBy, &b.CreatedAt, &b.ItemCount,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get bundle: %w", err)
	}
	if desc != nil {
		b.Description = *desc
	}
	return b, nil
}

// ListBundles lists all evidence bundles for a tenant
func (r *Repo) ListBundles(ctx context.Context, tenantID uuid.UUID) ([]*EvidenceBundle, error) {
	query := `
		SELECT b.id, b.tenant_id, b.name, b.description, b.created_by, b.created_at,
		       (SELECT COUNT(*) FROM evidence_bundle_items WHERE bundle_id = b.id)
		FROM evidence_bundles b
		WHERE b.tenant_id = $1
		ORDER BY b.created_at DESC`
	rows, err := r.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list bundles: %w", err)
	}
	defer rows.Close()

	var bundles []*EvidenceBundle
	for rows.Next() {
		b := &EvidenceBundle{}
		var desc *string
		if err := rows.Scan(&b.ID, &b.TenantID, &b.Name, &desc, &b.CreatedBy, &b.CreatedAt, &b.ItemCount); err != nil {
			return nil, fmt.Errorf("scan bundle: %w", err)
		}
		if desc != nil {
			b.Description = *desc
		}
		bundles = append(bundles, b)
	}
	return bundles, rows.Err()
}

// DeleteBundle deletes a bundle and its items
func (r *Repo) DeleteBundle(ctx context.Context, tenantID, id uuid.UUID) error {
	query := `DELETE FROM evidence_bundles WHERE tenant_id = $1 AND id = $2`
	_, err := r.db.Exec(ctx, query, tenantID, id)
	return err
}

// AddBundleItem adds an attachment to a bundle
func (r *Repo) AddBundleItem(ctx context.Context, item *EvidenceBundleItem) error {
	query := `
		INSERT INTO evidence_bundle_items (tenant_id, bundle_id, attachment_id, note)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (bundle_id, attachment_id) DO UPDATE SET note = EXCLUDED.note
		RETURNING id, created_at`
	return r.db.QueryRow(ctx, query,
		item.TenantID, item.BundleID, item.AttachmentID, item.Note,
	).Scan(&item.ID, &item.CreatedAt)
}

// RemoveBundleItem removes an attachment from a bundle
func (r *Repo) RemoveBundleItem(ctx context.Context, tenantID, bundleID, attachmentID uuid.UUID) error {
	query := `DELETE FROM evidence_bundle_items WHERE tenant_id = $1 AND bundle_id = $2 AND attachment_id = $3`
	_, err := r.db.Exec(ctx, query, tenantID, bundleID, attachmentID)
	return err
}

// ListBundleItems lists all items in a bundle with attachment details
func (r *Repo) ListBundleItems(ctx context.Context, tenantID, bundleID uuid.UUID) ([]*EvidenceBundleItem, error) {
	query := `
		SELECT i.id, i.tenant_id, i.bundle_id, i.attachment_id, i.note, i.created_at,
		       a.id, a.tenant_id, a.filename, a.content_type, a.size_bytes, a.sha256, a.storage_key, a.created_by, a.created_at
		FROM evidence_bundle_items i
		JOIN attachments a ON a.id = i.attachment_id AND a.tenant_id = i.tenant_id
		WHERE i.tenant_id = $1 AND i.bundle_id = $2
		ORDER BY i.created_at`
	rows, err := r.db.Query(ctx, query, tenantID, bundleID)
	if err != nil {
		return nil, fmt.Errorf("list bundle items: %w", err)
	}
	defer rows.Close()

	var items []*EvidenceBundleItem
	for rows.Next() {
		item := &EvidenceBundleItem{Attachment: &Attachment{}}
		var note *string
		if err := rows.Scan(
			&item.ID, &item.TenantID, &item.BundleID, &item.AttachmentID, &note, &item.CreatedAt,
			&item.Attachment.ID, &item.Attachment.TenantID, &item.Attachment.Filename, &item.Attachment.ContentType,
			&item.Attachment.SizeBytes, &item.Attachment.SHA256, &item.Attachment.StorageKey,
			&item.Attachment.CreatedBy, &item.Attachment.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan bundle item: %w", err)
		}
		if note != nil {
			item.Note = *note
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// DeleteAttachment removes an attachment if it has no links or bundle references.
// Returns the storage key so the caller can delete the file.
func (r *Repo) DeleteAttachment(ctx context.Context, tenantID, attachmentID uuid.UUID) (storageKey string, err error) {
	// Check for remaining links
	var linkCount int
	err = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM attachment_links WHERE tenant_id = $1 AND attachment_id = $2`,
		tenantID, attachmentID).Scan(&linkCount)
	if err != nil {
		return "", fmt.Errorf("check links: %w", err)
	}
	if linkCount > 0 {
		return "", fmt.Errorf("attachment still has %d link(s)", linkCount)
	}

	// Check for bundle references
	var bundleCount int
	err = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM evidence_bundle_items WHERE tenant_id = $1 AND attachment_id = $2`,
		tenantID, attachmentID).Scan(&bundleCount)
	if err != nil {
		return "", fmt.Errorf("check bundles: %w", err)
	}
	if bundleCount > 0 {
		return "", fmt.Errorf("attachment still in %d bundle(s)", bundleCount)
	}

	// Get storage key and delete
	err = r.db.QueryRow(ctx, `DELETE FROM attachments WHERE tenant_id = $1 AND id = $2 RETURNING storage_key`,
		tenantID, attachmentID).Scan(&storageKey)
	if err != nil {
		return "", fmt.Errorf("delete attachment: %w", err)
	}
	return storageKey, nil
}

// CountOrphanedAttachments returns the number of attachments with no links or bundle references.
func (r *Repo) CountOrphanedAttachments(ctx context.Context, tenantID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM attachments a
		WHERE a.tenant_id = $1
		  AND NOT EXISTS (SELECT 1 FROM attachment_links al WHERE al.attachment_id = a.id AND al.tenant_id = a.tenant_id)
		  AND NOT EXISTS (SELECT 1 FROM evidence_bundle_items ebi WHERE ebi.attachment_id = a.id AND ebi.tenant_id = a.tenant_id)
	`, tenantID).Scan(&count)
	return count, err
}

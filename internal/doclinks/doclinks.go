// Package doclinks manages internal document cross-references.
package doclinks

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Link represents a cross-reference from one document to another.
type Link struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	FromDocumentID uuid.UUID
	ToDocumentID   *uuid.UUID
	LinkPath       string
	Broken         bool
}

// BacklinkDoc is a document that links to the current doc.
type BacklinkDoc struct {
	DocumentID uuid.UUID
	Path       string
	Title      string
}

// BrokenLink is a link that doesn't resolve to any document.
type BrokenLink struct {
	FromDocumentID uuid.UUID
	FromPath       string
	FromTitle      string
	LinkPath       string
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// internalLinkRe matches markdown links like [text](/docs/path) or [text](path).
// We only care about links that look like internal doc references.
var internalLinkRe = regexp.MustCompile(`\]\((/docs/[^)\s]+|[^)\s:]+\.md)\)`)

// ParseLinks extracts internal document link paths from markdown source.
func ParseLinks(markdown string) []string {
	matches := internalLinkRe.FindAllStringSubmatch(markdown, -1)
	seen := make(map[string]struct{})
	var paths []string
	for _, m := range matches {
		raw := m[1]
		// Normalize: strip /docs/ prefix, strip .md suffix, strip anchor
		p := NormalizeLinkPath(raw)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	return paths
}

// NormalizeLinkPath converts a raw link href to a canonical doc path.
func NormalizeLinkPath(raw string) string {
	// Strip /docs/ prefix
	p := strings.TrimPrefix(raw, "/docs/")
	// Strip .md suffix
	p = strings.TrimSuffix(p, ".md")
	// Strip anchor
	if idx := strings.Index(p, "#"); idx >= 0 {
		p = p[:idx]
	}
	// Strip query
	if idx := strings.Index(p, "?"); idx >= 0 {
		p = p[:idx]
	}
	// Clean path
	p = path.Clean(p)
	// Strip leading slash
	p = strings.TrimPrefix(p, "/")
	if p == "." || p == "" {
		return ""
	}
	return p
}

// ResolveRelative resolves a relative link path against the directory of the source document.
func ResolveRelative(fromDocPath, linkPath string) string {
	if strings.HasPrefix(linkPath, "/") {
		return strings.TrimPrefix(linkPath, "/")
	}
	dir := path.Dir(fromDocPath)
	if dir == "." {
		dir = ""
	}
	resolved := path.Join(dir, linkPath)
	return path.Clean(resolved)
}

// RebuildLinks rebuilds the doc_links table for a given document.
// Called after each save/revision.
func (r *Repository) RebuildLinks(ctx context.Context, tenantID, fromDocID uuid.UUID, fromDocPath, markdown string) error {
	// Parse link paths from markdown
	linkPaths := ParseLinks(markdown)

	// Delete existing links from this document
	_, err := r.db.Exec(ctx,
		`DELETE FROM doc_links WHERE tenant_id = $1 AND from_document_id = $2`,
		tenantID, fromDocID)
	if err != nil {
		return fmt.Errorf("delete old links: %w", err)
	}

	if len(linkPaths) == 0 {
		return nil
	}

	// Resolve each link and check if target exists
	for _, lp := range linkPaths {
		// Resolve relative links
		resolved := lp
		if !strings.HasPrefix(lp, "/") && !strings.Contains(lp, "/") {
			// Could be relative — try resolving
			resolved = ResolveRelative(fromDocPath, lp)
		}

		// Look up target document by path
		var toDocID *uuid.UUID
		broken := true
		var targetID uuid.UUID
		err := r.db.QueryRow(ctx,
			`SELECT id FROM documents WHERE tenant_id = $1 AND path = $2`,
			tenantID, resolved).Scan(&targetID)
		if err == nil {
			toDocID = &targetID
			broken = false
		}

		_, err = r.db.Exec(ctx,
			`INSERT INTO doc_links (tenant_id, from_document_id, to_document_id, link_path, broken)
			 VALUES ($1, $2, $3, $4, $5)`,
			tenantID, fromDocID, toDocID, resolved, broken)
		if err != nil {
			return fmt.Errorf("insert link %s: %w", resolved, err)
		}
	}

	return nil
}

// GetBacklinks returns documents that link TO the given document.
func (r *Repository) GetBacklinks(ctx context.Context, tenantID, docID uuid.UUID) ([]BacklinkDoc, error) {
	rows, err := r.db.Query(ctx, `
		SELECT d.id, d.path, d.title
		FROM doc_links dl
		JOIN documents d ON dl.from_document_id = d.id AND d.tenant_id = dl.tenant_id
		WHERE dl.tenant_id = $1 AND dl.to_document_id = $2
		ORDER BY d.title
	`, tenantID, docID)
	if err != nil {
		return nil, fmt.Errorf("query backlinks: %w", err)
	}
	defer rows.Close()

	var backlinks []BacklinkDoc
	for rows.Next() {
		var bl BacklinkDoc
		if err := rows.Scan(&bl.DocumentID, &bl.Path, &bl.Title); err != nil {
			return nil, fmt.Errorf("scan backlink: %w", err)
		}
		backlinks = append(backlinks, bl)
	}
	return backlinks, rows.Err()
}

// GetBrokenLinks returns all broken links within a tenant.
func (r *Repository) GetBrokenLinks(ctx context.Context, tenantID uuid.UUID) ([]BrokenLink, error) {
	rows, err := r.db.Query(ctx, `
		SELECT dl.from_document_id, d.path, d.title, dl.link_path
		FROM doc_links dl
		JOIN documents d ON dl.from_document_id = d.id AND d.tenant_id = dl.tenant_id
		WHERE dl.tenant_id = $1 AND dl.broken = TRUE
		ORDER BY d.title, dl.link_path
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("query broken links: %w", err)
	}
	defer rows.Close()

	var links []BrokenLink
	for rows.Next() {
		var bl BrokenLink
		if err := rows.Scan(&bl.FromDocumentID, &bl.FromPath, &bl.FromTitle, &bl.LinkPath); err != nil {
			return nil, fmt.Errorf("scan broken link: %w", err)
		}
		links = append(links, bl)
	}
	return links, rows.Err()
}

// MarkLinksToPathBroken marks all links pointing to a given path as broken.
// Used after document rename/delete.
func (r *Repository) MarkLinksToPathBroken(ctx context.Context, tenantID uuid.UUID, docPath string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE doc_links SET broken = TRUE, to_document_id = NULL
		 WHERE tenant_id = $1 AND link_path = $2`,
		tenantID, docPath)
	return err
}

// DeleteLinksFrom deletes all outgoing links from a document.
func (r *Repository) DeleteLinksFrom(ctx context.Context, tenantID, docID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM doc_links WHERE tenant_id = $1 AND from_document_id = $2`,
		tenantID, docID)
	return err
}

// CountBrokenLinks returns the count of broken links for a tenant.
func (r *Repository) CountBrokenLinks(ctx context.Context, tenantID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM doc_links WHERE tenant_id = $1 AND broken = TRUE`,
		tenantID).Scan(&count)
	return count, err
}

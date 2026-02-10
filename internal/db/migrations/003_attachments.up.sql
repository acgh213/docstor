-- Phase 8: Attachments + Evidence Bundles

-- Attachments table: stores file metadata
CREATE TABLE attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    sha256 TEXT NOT NULL,
    storage_key TEXT NOT NULL,  -- path on disk or S3 key
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_attachments_tenant ON attachments(tenant_id);
CREATE INDEX idx_attachments_sha256 ON attachments(tenant_id, sha256);

-- Attachment links: polymorphic links to docs, revisions, incidents, etc.
CREATE TABLE attachment_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    attachment_id UUID NOT NULL REFERENCES attachments(id) ON DELETE CASCADE,
    linked_type TEXT NOT NULL,  -- 'document' | 'revision' | 'incident' | 'change'
    linked_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_attachment_links_tenant ON attachment_links(tenant_id);
CREATE INDEX idx_attachment_links_attachment ON attachment_links(attachment_id);
CREATE INDEX idx_attachment_links_linked ON attachment_links(tenant_id, linked_type, linked_id);

-- Evidence bundles: collections of attachments for export
CREATE TABLE evidence_bundles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_evidence_bundles_tenant ON evidence_bundles(tenant_id);

-- Evidence bundle items: attachments in a bundle
CREATE TABLE evidence_bundle_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    bundle_id UUID NOT NULL REFERENCES evidence_bundles(id) ON DELETE CASCADE,
    attachment_id UUID NOT NULL REFERENCES attachments(id) ON DELETE CASCADE,
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(bundle_id, attachment_id)
);

CREATE INDEX idx_evidence_bundle_items_bundle ON evidence_bundle_items(bundle_id);

-- Doc links: tracks internal links between documents
CREATE TABLE doc_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    from_document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    to_document_id UUID REFERENCES documents(id) ON DELETE SET NULL,
    link_path TEXT NOT NULL,  -- the raw link path from markdown
    broken BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_doc_links_from ON doc_links(tenant_id, from_document_id);
CREATE INDEX idx_doc_links_to ON doc_links(tenant_id, to_document_id);
CREATE INDEX idx_doc_links_broken ON doc_links(tenant_id, broken) WHERE broken = TRUE;

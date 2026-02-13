-- Change Records: track planned changes with risk assessment and status workflow
CREATE TABLE changes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    client_id UUID REFERENCES clients(id),
    title TEXT NOT NULL,
    description_markdown TEXT NOT NULL DEFAULT '',
    risk_level TEXT NOT NULL DEFAULT 'low' CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'approved', 'in_progress', 'completed', 'rolled_back', 'cancelled')),
    window_start TIMESTAMPTZ,
    window_end TIMESTAMPTZ,
    rollback_plan_markdown TEXT NOT NULL DEFAULT '',
    validation_plan_markdown TEXT NOT NULL DEFAULT '',
    created_by UUID NOT NULL REFERENCES users(id),
    approved_by UUID REFERENCES users(id),
    approved_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_changes_tenant ON changes(tenant_id);
CREATE INDEX idx_changes_status ON changes(tenant_id, status);
CREATE INDEX idx_changes_client ON changes(tenant_id, client_id);

-- Link changes to other entities
CREATE TABLE change_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    change_id UUID NOT NULL REFERENCES changes(id) ON DELETE CASCADE,
    linked_type TEXT NOT NULL CHECK (linked_type IN ('document', 'checklist_instance', 'evidence_bundle')),
    linked_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_change_links_change ON change_links(tenant_id, change_id);
CREATE UNIQUE INDEX idx_change_links_unique ON change_links(change_id, linked_type, linked_id);

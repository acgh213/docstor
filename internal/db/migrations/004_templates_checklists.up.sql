-- Phase 9: Templates + Checklists

-- Templates: reusable doc/runbook starters
CREATE TABLE templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    template_type TEXT NOT NULL DEFAULT 'doc' CHECK (template_type IN ('doc', 'runbook')),
    body_markdown TEXT NOT NULL DEFAULT '',
    default_metadata_json JSONB DEFAULT '{}',
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_templates_tenant ON templates(tenant_id);
CREATE INDEX idx_templates_tenant_type ON templates(tenant_id, template_type);

-- Checklists: reusable checklist definitions
CREATE TABLE checklists (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_checklists_tenant ON checklists(tenant_id);

-- Checklist items: individual items in a checklist definition
CREATE TABLE checklist_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    checklist_id UUID NOT NULL REFERENCES checklists(id) ON DELETE CASCADE,
    position INT NOT NULL DEFAULT 0,
    text TEXT NOT NULL
);

CREATE INDEX idx_checklist_items_checklist ON checklist_items(checklist_id, position);

-- Checklist instances: a started checklist, optionally linked to a document
CREATE TABLE checklist_instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    checklist_id UUID NOT NULL REFERENCES checklists(id) ON DELETE CASCADE,
    linked_type TEXT,          -- 'document' or NULL
    linked_id UUID,            -- document id or NULL
    status TEXT NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'completed')),
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_checklist_instances_tenant ON checklist_instances(tenant_id);
CREATE INDEX idx_checklist_instances_linked ON checklist_instances(linked_type, linked_id);
CREATE INDEX idx_checklist_instances_status ON checklist_instances(tenant_id, status);

-- Checklist instance items: tracked completion of each item
CREATE TABLE checklist_instance_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    instance_id UUID NOT NULL REFERENCES checklist_instances(id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES checklist_items(id) ON DELETE CASCADE,
    done BOOLEAN NOT NULL DEFAULT FALSE,
    done_by_user_id UUID REFERENCES users(id),
    done_at TIMESTAMPTZ,
    note TEXT,
    UNIQUE(instance_id, item_id)
);

CREATE INDEX idx_checklist_instance_items_instance ON checklist_instance_items(instance_id);

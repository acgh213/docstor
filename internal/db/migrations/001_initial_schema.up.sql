-- Tenants
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMPTZ
);

-- Memberships (tenant-user-role relationships)
CREATE TABLE memberships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('admin', 'editor', 'reader')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, user_id)
);

CREATE INDEX idx_memberships_tenant ON memberships(tenant_id);
CREATE INDEX idx_memberships_user ON memberships(user_id);

-- Clients (within a tenant)
CREATE TABLE clients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    code TEXT NOT NULL,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, code)
);

CREATE INDEX idx_clients_tenant ON clients(tenant_id);

-- Documents
CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    client_id UUID REFERENCES clients(id) ON DELETE SET NULL,
    path TEXT NOT NULL,
    title TEXT NOT NULL,
    doc_type TEXT NOT NULL DEFAULT 'doc' CHECK (doc_type IN ('doc', 'runbook')),
    sensitivity TEXT NOT NULL DEFAULT 'public-internal' CHECK (sensitivity IN ('public-internal', 'restricted', 'confidential')),
    owner_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    metadata_json JSONB DEFAULT '{}',
    current_revision_id UUID,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, path)
);

CREATE INDEX idx_documents_tenant ON documents(tenant_id);
CREATE INDEX idx_documents_tenant_client ON documents(tenant_id, client_id);
CREATE INDEX idx_documents_tenant_updated ON documents(tenant_id, updated_at DESC);

-- Revisions
CREATE TABLE revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    body_markdown TEXT NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    message TEXT,
    base_revision_id UUID REFERENCES revisions(id)
);

CREATE INDEX idx_revisions_document ON revisions(document_id);
CREATE INDEX idx_revisions_tenant ON revisions(tenant_id);

-- Add foreign key for current_revision_id after revisions table exists
ALTER TABLE documents
    ADD CONSTRAINT fk_documents_current_revision
    FOREIGN KEY (current_revision_id)
    REFERENCES revisions(id);

-- Audit log
CREATE TABLE audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id UUID,
    at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ip TEXT,
    user_agent TEXT,
    metadata_json JSONB DEFAULT '{}'
);

CREATE INDEX idx_audit_log_tenant ON audit_log(tenant_id);
CREATE INDEX idx_audit_log_tenant_at ON audit_log(tenant_id, at DESC);
CREATE INDEX idx_audit_log_target ON audit_log(target_type, target_id);

-- Runbook status (for verification tracking)
CREATE TABLE runbook_status (
    document_id UUID PRIMARY KEY REFERENCES documents(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    last_verified_at TIMESTAMPTZ,
    last_verified_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    verification_interval_days INT NOT NULL DEFAULT 90,
    next_due_at TIMESTAMPTZ
);

CREATE INDEX idx_runbook_status_tenant ON runbook_status(tenant_id);
CREATE INDEX idx_runbook_status_next_due ON runbook_status(tenant_id, next_due_at);

-- Sessions table for auth
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    ip TEXT,
    user_agent TEXT
);

CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_token ON sessions(token_hash);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

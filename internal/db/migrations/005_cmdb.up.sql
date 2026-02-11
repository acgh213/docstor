-- Phase 10: CMDB-lite

CREATE TABLE systems (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    client_id UUID REFERENCES clients(id),
    system_type TEXT NOT NULL DEFAULT 'server',
    name TEXT NOT NULL,
    fqdn TEXT,
    ip TEXT,
    os TEXT,
    environment TEXT NOT NULL DEFAULT 'production',
    notes TEXT,
    owner_user_id UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_systems_tenant ON systems(tenant_id);
CREATE INDEX idx_systems_client ON systems(tenant_id, client_id);

CREATE TABLE vendors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    client_id UUID REFERENCES clients(id),
    name TEXT NOT NULL,
    vendor_type TEXT NOT NULL DEFAULT 'general',
    phone TEXT,
    email TEXT,
    portal_url TEXT,
    escalation_notes TEXT,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_vendors_tenant ON vendors(tenant_id);
CREATE INDEX idx_vendors_client ON vendors(tenant_id, client_id);

CREATE TABLE contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    client_id UUID REFERENCES clients(id),
    name TEXT NOT NULL,
    role TEXT,
    phone TEXT,
    email TEXT,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_contacts_tenant ON contacts(tenant_id);
CREATE INDEX idx_contacts_client ON contacts(tenant_id, client_id);

CREATE TABLE circuits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    client_id UUID REFERENCES clients(id),
    provider TEXT NOT NULL,
    circuit_id TEXT NOT NULL,
    circuit_type TEXT NOT NULL DEFAULT 'internet',
    wan_ip TEXT,
    speed TEXT,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_circuits_tenant ON circuits(tenant_id);
CREATE INDEX idx_circuits_client ON circuits(tenant_id, client_id);

-- Phase 13: Sites (client -> sites)

CREATE TABLE sites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    client_id UUID NOT NULL REFERENCES clients(id),
    name TEXT NOT NULL,
    address TEXT,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sites_tenant ON sites(tenant_id);
CREATE INDEX idx_sites_client ON sites(tenant_id, client_id);

-- Add optional site_id to CMDB tables
ALTER TABLE systems ADD COLUMN site_id UUID REFERENCES sites(id);
CREATE INDEX idx_systems_site ON systems(tenant_id, site_id);

ALTER TABLE contacts ADD COLUMN site_id UUID REFERENCES sites(id);
CREATE INDEX idx_contacts_site ON contacts(tenant_id, site_id);

ALTER TABLE circuits ADD COLUMN site_id UUID REFERENCES sites(id);
CREATE INDEX idx_circuits_site ON circuits(tenant_id, site_id);

-- Rollback Phase 13: Sites

DROP INDEX IF EXISTS idx_circuits_site;
ALTER TABLE circuits DROP COLUMN IF EXISTS site_id;

DROP INDEX IF EXISTS idx_contacts_site;
ALTER TABLE contacts DROP COLUMN IF EXISTS site_id;

DROP INDEX IF EXISTS idx_systems_site;
ALTER TABLE systems DROP COLUMN IF EXISTS site_id;

DROP TABLE IF EXISTS sites;

-- Remove full-text search support

DROP TRIGGER IF EXISTS trg_documents_search_vector ON documents;
DROP FUNCTION IF EXISTS update_document_search_vector();
DROP INDEX IF EXISTS idx_documents_search;
ALTER TABLE documents DROP COLUMN IF EXISTS search_vector;

-- Add full-text search support to documents

-- Add search vector column
ALTER TABLE documents ADD COLUMN search_vector tsvector;

-- Create GIN index for fast search
CREATE INDEX idx_documents_search ON documents USING GIN(search_vector);

-- Function to update search vector
-- Combines title, path, and current revision body
CREATE OR REPLACE FUNCTION update_document_search_vector()
RETURNS TRIGGER AS $$
DECLARE
    doc_body TEXT;
BEGIN
    -- Get the body from current revision
    SELECT body_markdown INTO doc_body
    FROM revisions
    WHERE id = NEW.current_revision_id;
    
    -- Update search vector with weighted components
    -- A = highest weight (title), B = path, C/D = body
    NEW.search_vector := 
        setweight(to_tsvector('english', COALESCE(NEW.title, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.path, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(doc_body, '')), 'C');
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to update search vector on document changes
CREATE TRIGGER trg_documents_search_vector
    BEFORE INSERT OR UPDATE OF title, path, current_revision_id
    ON documents
    FOR EACH ROW
    EXECUTE FUNCTION update_document_search_vector();

-- Backfill existing documents
UPDATE documents SET search_vector = (
    setweight(to_tsvector('english', COALESCE(title, '')), 'A') ||
    setweight(to_tsvector('english', COALESCE(path, '')), 'B') ||
    setweight(to_tsvector('english', COALESCE(
        (SELECT body_markdown FROM revisions WHERE id = documents.current_revision_id),
        ''
    )), 'C')
);

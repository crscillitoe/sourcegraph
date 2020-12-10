BEGIN;

ALTER TABLE lsif_nearest_uploads ADD COLUMN ancestor_visible boolean;
ALTER TABLE lsif_nearest_uploads ADD COLUMN overwritten boolean;
UPDATE lsif_nearest_uploads SET ancestor_visible = true, overwritten = false;
ALTER TABLE lsif_nearest_uploads ALTER COLUMN ancestor_visible SET NOT NULL;
ALTER TABLE lsif_nearest_uploads ALTER COLUMN overwritten SET NOT NULL;

DROP TABLE IF EXISTS lsif_nearest_uploads_links;

COMMIT;

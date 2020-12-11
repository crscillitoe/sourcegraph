BEGIN;

ALTER TABLE lsif_nearest_uploads ADD COLUMN ancestor_visible boolean;
ALTER TABLE lsif_nearest_uploads ADD COLUMN overwritten boolean;
UPDATE lsif_nearest_uploads SET ancestor_visible = true, overwritten = false;
ALTER TABLE lsif_nearest_uploads ALTER COLUMN ancestor_visible SET NOT NULL;
ALTER TABLE lsif_nearest_uploads ALTER COLUMN overwritten SET NOT NULL;

DROP TABLE IF EXISTS lsif_nearest_uploads_links;

DROP INDEX lsif_uploads_visible_at_tip_repository_id_upload_id;
CREATE INDEX lsif_uploads_visible_at_tip_repository_id ON lsif_uploads_visible_at_tip(repository_id);

COMMIT;

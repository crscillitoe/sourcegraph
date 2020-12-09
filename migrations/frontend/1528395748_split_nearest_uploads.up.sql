BEGIN;

CREATE TABLE IF NOT EXISTS lsif_nearest_uploads_links (
    repository_id int NOT NULL,
    commit_bytea bytea NOT NULL,
    ancestor_commit_bytea bytea,
    ancestor_distance int,
    descendant_commit_bytea bytea,
    descendant_distance int
);

CREATE INDEX lsif_nearest_uploads_links_repository_id_commit_bytea ON lsif_nearest_uploads_links USING btree (repository_id, commit_bytea);

COMMIT;

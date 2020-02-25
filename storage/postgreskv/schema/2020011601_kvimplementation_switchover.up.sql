DROP VIEW pathdata_pretty;
DROP FUNCTION list_directory_reverse(BYTEA, BYTEA, BYTEA, INTEGER);
DROP FUNCTION list_directory(BYTEA, BYTEA, BYTEA, INTEGER);
DROP FUNCTION component_increment(BYTEA, INTEGER);
DROP FUNCTION bytea_increment(BYTEA);
DROP FUNCTION truncate_after(BYTEA, INTEGER, INTEGER);
DROP TYPE path_and_meta;
CREATE TABLE IF NOT EXISTS pathdata_v2 (
                                        fullpath BYTEA PRIMARY KEY,
                                        metadata BYTEA NOT NULL
);
ALTER TABLE pathdata_v2 ALTER COLUMN fullpath SET STORAGE MAIN;
ALTER TABLE pathdata_v2 ALTER COLUMN metadata SET STORAGE MAIN;
INSERT INTO pathdata_v2 (fullpath, metadata) SELECT fullpath, metadata FROM pathdata;
DROP TABLE pathdata;
DROP TABLE buckets;
ALTER TABLE pathdata_v2 RENAME TO pathdata;

CREATE TABLE IF NOT EXISTS pathdata_v2 (
                                        fullpath BYTEA PRIMARY KEY,
                                        metadata BYTEA NOT NULL
);
ALTER TABLE pathdata_v2 ALTER COLUMN fullpath SET STORAGE PLAIN;
ALTER TABLE pathdata_v2 ALTER COLUMN metadata SET STORAGE PLAIN;
INSERT INTO pathdata_v2 (fullpath, metadata) SELECT fullpath, metadata FROM pathdata;
DROP TABLE pathdata;
ALTER TABLE pathdata_v2 RENAME TO pathdata;
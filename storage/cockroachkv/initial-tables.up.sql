CREATE TABLE buckets (
	bucketname BYTES NOT NULL,
	delim INT8 NOT NULL,
	CONSTRAINT "primary" PRIMARY KEY (bucketname ASC),
	FAMILY "primary" (bucketname, delim),
	CONSTRAINT check_delim_delim CHECK ((delim > 0) AND (delim < 255))
);

CREATE TABLE pathdata (
	bucket BYTES NOT NULL,
	fullpath BYTES NOT NULL,
	metadata BYTES NOT NULL,
	CONSTRAINT "primary" PRIMARY KEY (bucket ASC, fullpath ASC),
	FAMILY "primary" (bucket, fullpath, metadata),
	CONSTRAINT check_fullpath CHECK (fullpath != '')
);

PREPARE prepared_stmt AS DELETE FROM pathdata
	USING matching_key mk
	WHERE pathdata.metadata = $3::BYTEA
		AND pathdata.bucket = mk.bucket
		AND pathdata.fullpath = mk.fullpath
	RETURNING 1
)
SELECT EXISTS(SELECT 1 FROM matching_key) AS key_present, EXISTS(SELECT 1 FROM updated) AS value_updated
PREPARE;
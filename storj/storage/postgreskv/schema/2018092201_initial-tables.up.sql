CREATE TABLE buckets (
    bucketname BYTEA
        PRIMARY KEY,
    delim INT
        NOT NULL
        CHECK (delim > 0 AND delim < 255)
);

-- until the KeyValueStore interface supports passing the bucket separately, or
-- until storj actually supports changing the delimiter character per bucket, this
-- dummy row should suffice for everything.
INSERT INTO buckets (bucketname, delim) VALUES (''::BYTEA, ascii('/'));


CREATE TABLE pathdata (
    bucket BYTEA
        NOT NULL
        REFERENCES buckets (bucketname),
    fullpath BYTEA
        NOT NULL
        CHECK (fullpath <> ''),
    metadata BYTEA
        NOT NULL,

    PRIMARY KEY (bucket, fullpath)
);

CREATE VIEW pathdata_pretty AS
    SELECT encode(bucket, 'escape') AS bucket,
           encode(fullpath, 'escape') AS fullpath,
           encode(metadata, 'escape') AS metadata
      FROM pathdata;


-- given a path as might be found in the pathdata table, truncate it after the next delimiter to be
-- found at or after 'afterpos', if any.
--
-- Examples:
--
--     truncate_after(''::BYTEA,            ascii('/'), 1)  ->  ''::BYTEA
--     truncate_after('foo'::BYTEA,         ascii('/'), 1)  ->  'foo'::BYTEA
--     truncate_after('foo/'::BYTEA,        ascii('/'), 1)  ->  'foo/'::BYTEA
--     truncate_after('foo/bar/baz'::BYTEA, ascii('/'), 4)  ->  'foo/'::BYTEA
--     truncate_after('foo/bar/baz'::BYTEA, ascii('/'), 5)  ->  'foo/bar/'::BYTEA
--     truncate_after('foo/bar/baz'::BYTEA, ascii('/'), 8)  ->  'foo/bar/'::BYTEA
--     truncate_after('foo/bar/baz'::BYTEA, ascii('/'), 9)  ->  'foo/bar/baz'::BYTEA
--     truncate_after('foo//bar/bz'::BYTEA, ascii('/'), 4)  ->  'foo/'::BYTEA
--     truncate_after('foo//bar/bz'::BYTEA, ascii('/'), 5)  ->  'foo//'::BYTEA
--
CREATE FUNCTION truncate_after(bpath BYTEA, delim INTEGER, afterpos INTEGER) RETURNS BYTEA AS $$
DECLARE
    suff BYTEA;
    delimpos INTEGER;
BEGIN
    suff := substring(bpath FROM afterpos);
    delimpos := position(set_byte(' '::BYTEA, 0, delim) IN suff);
    IF delimpos > 0 THEN
        RETURN substring(bpath FROM 1 FOR (afterpos + delimpos - 1));
    END IF;
    RETURN bpath;
END;
$$ LANGUAGE 'plpgsql' IMMUTABLE STRICT;


CREATE FUNCTION bytea_increment(b BYTEA) RETURNS BYTEA AS $$
BEGIN
    WHILE b <> ''::BYTEA AND get_byte(b, octet_length(b) - 1) = 255 LOOP
        b := substring(b FROM 1 FOR octet_length(b) - 1);
    END LOOP;
    IF b = ''::BYTEA THEN
        RETURN NULL;
    END IF;
    RETURN set_byte(b, octet_length(b) - 1, get_byte(b, octet_length(b) - 1) + 1);
END;
$$ LANGUAGE 'plpgsql' IMMUTABLE STRICT;


-- Given a path as might be found in the pathdata table, with a delimeter appended if that path
-- has any sub-elements, return the next possible path that _could_ be in the table (skipping over
-- any potential sub-elements).
--
-- Examples:
--
--     component_increment('/'::BYTEA, ascii('/'))  ->  '0'::BYTEA
--         (nothing can be between '/' and '0' other than subpaths under '/')
--
--     component_increment('/foo/bar/'::BYTEA, ascii('/'))  ->  '/foo/bar0'::BYTEA
--
--     component_increment('/foo/barboom'::BYTEA, ascii('/'))  ->  ('/foo/barboom' || E'\\x00')
--         (nothing can be between '/foo/barboom' and '/foo/barboom\x00' in normal BYTEA ordering)
--
--     component_increment(E'\\xFEFFFF'::BYTEA, 255)  ->  E'\\xFF'::BYTEA
--
CREATE FUNCTION component_increment(bpath BYTEA, delim INTEGER) RETURNS BYTEA AS $$
    SELECT CASE WHEN get_byte(bpath, octet_length(bpath) - 1) = delim
                THEN CASE WHEN delim = 255
                          THEN bytea_increment(bpath)
                          ELSE set_byte(bpath, octet_length(bpath) - 1, delim + 1)
                     END
                ELSE bpath || E'\\x00'::BYTEA
           END;
$$ LANGUAGE 'sql' IMMUTABLE STRICT;


CREATE TYPE path_and_meta AS (
    fullpath BYTEA,
    metadata BYTEA
);

CREATE FUNCTION list_directory(bucket BYTEA, dirpath BYTEA, start_at BYTEA = ''::BYTEA, limit_to INTEGER = NULL)
RETURNS SETOF path_and_meta AS $$
    WITH RECURSIVE
        inputs AS (
            SELECT CASE WHEN dirpath = ''::BYTEA THEN NULL ELSE dirpath END AS range_low,
                   CASE WHEN dirpath = ''::BYTEA THEN NULL ELSE bytea_increment(dirpath) END AS range_high,
                   octet_length(dirpath) + 1 AS component_start,
                   b.delim AS delim,
                   b.bucketname AS bucket
              FROM buckets b
             WHERE bucketname = bucket
        ),
        distinct_prefix (truncatedpath) AS (
            SELECT (SELECT truncate_after(pd.fullpath, i.delim, i.component_start)
                      FROM pathdata pd
                     WHERE (i.range_low IS NULL OR pd.fullpath > i.range_low)
                       AND (i.range_high IS NULL OR pd.fullpath < i.range_high)
                       AND (start_at = '' OR pd.fullpath >= start_at)
                       AND pd.bucket = i.bucket
                     ORDER BY pd.fullpath
                     LIMIT 1)
              FROM inputs i
            UNION ALL
            SELECT (SELECT truncate_after(pd.fullpath, i.delim, i.component_start)
                      FROM pathdata pd
                     WHERE pd.fullpath >= component_increment(pfx.truncatedpath, i.delim)
                       AND (i.range_high IS NULL OR pd.fullpath < i.range_high)
                       AND pd.bucket = i.bucket
                     ORDER BY pd.fullpath
                     LIMIT 1)
              FROM distinct_prefix pfx, inputs i
             WHERE pfx.truncatedpath IS NOT NULL
        )
    SELECT pfx.truncatedpath AS fullpath,
           pd.metadata
      FROM distinct_prefix pfx LEFT OUTER JOIN pathdata pd ON pfx.truncatedpath = pd.fullpath
     WHERE pfx.truncatedpath IS NOT NULL
    UNION ALL
    -- this one, if it exists, can't be part of distinct_prefix (or it would cause us to skip over all
    -- subcontents of the prefix we're looking for), so we tack it on here
    SELECT pd.fullpath, pd.metadata FROM pathdata pd, inputs i WHERE pd.fullpath = i.range_low
     ORDER BY fullpath
     LIMIT limit_to;
$$ LANGUAGE 'sql' STABLE;

CREATE FUNCTION list_directory_reverse(bucket BYTEA, dirpath BYTEA, start_at BYTEA = ''::BYTEA, limit_to INTEGER = NULL)
RETURNS SETOF path_and_meta AS $$
    WITH RECURSIVE
        inputs AS (
            SELECT CASE WHEN dirpath = ''::BYTEA THEN NULL ELSE dirpath END AS range_low,
                   CASE WHEN dirpath = ''::BYTEA THEN NULL ELSE bytea_increment(dirpath) END AS range_high,
                   octet_length(dirpath) + 1 AS component_start,
                   b.delim AS delim,
                   b.bucketname AS bucket
              FROM buckets b
             WHERE bucketname = bucket
        ),
        distinct_prefix (truncatedpath) AS (
            SELECT (SELECT truncate_after(pd.fullpath, i.delim, i.component_start)
                      FROM pathdata pd
                     WHERE (i.range_low IS NULL OR pd.fullpath >= i.range_low)
                       AND (i.range_high IS NULL OR pd.fullpath < i.range_high)
                       AND (start_at = '' OR pd.fullpath <= start_at)
                       AND pd.bucket = i.bucket
                     ORDER BY pd.fullpath DESC
                     LIMIT 1)
              FROM inputs i
            UNION ALL
            SELECT (SELECT truncate_after(pd.fullpath, i.delim, i.component_start)
                      FROM pathdata pd
                     WHERE (i.range_low IS NULL OR pd.fullpath >= i.range_low)
                       AND pd.fullpath < pfx.truncatedpath
                       AND pd.bucket = i.bucket
                     ORDER BY pd.fullpath DESC
                     LIMIT 1)
              FROM distinct_prefix pfx, inputs i
             WHERE pfx.truncatedpath IS NOT NULL
        )
    SELECT pfx.truncatedpath AS fullpath,
           pd.metadata
      FROM distinct_prefix pfx LEFT OUTER JOIN pathdata pd ON pfx.truncatedpath = pd.fullpath
     WHERE pfx.truncatedpath IS NOT NULL
     ORDER BY fullpath DESC
     LIMIT limit_to;
$$ LANGUAGE 'sql' STABLE;

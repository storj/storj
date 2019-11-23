CREATE OR REPLACE FUNCTION list_directory(bucket BYTEA, dirpath BYTEA, start_at BYTEA = ''::BYTEA, limit_to INTEGER = NULL)
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

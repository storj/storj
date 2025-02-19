-- A union of both the storage tally and bandwidth rollups.
-- Should be 1 row per project/bucket by partner within the timeframe specified
SELECT o.user_agent              as user_agent,
       o.project_id              as project_id,
       o.bucket_name             as bucket_name,
       SUM(o.total)              as total_byte_hours,
       SUM(o.remote)             as remote_byte_hours,
       SUM(o.inline)             as inline_byte_hours,
       SUM(o.segments)           as segment_hours,
       SUM(o.objects)            as object_hours,
       SUM(o.settled)            as settled,
       COALESCE(SUM(o.hours), 0) as hours
FROM (
         -- SUM the storage and hours
         -- Hours are used to calculate byte hours above
         SELECT bsti.user_agent                as user_agent,
                bsto.project_id                as project_id,
                bsto.bucket_name               as bucket_name,
                SUM(bsto.total_bytes)          as total,
                SUM(bsto.remote)               as remote,
                SUM(bsto.inline)               as inline,
                SUM(bsto.total_segments_count) as segments,
                SUM(bsto.object_count)         as objects,
                0                              as settled,
                count(1)                       as hours
         FROM (
                  -- Collapse entries by the latest record in the hour
                  -- If there are more than 1 records within the hour, only the latest will be considered
                  SELECT va.user_agent,
                         timestamp_trunc(bst.interval_start, hour, "UTC") as hours,
                         bst.project_id,
                         bst.bucket_name,
                         MAX(bst.interval_start)                as max_interval
                  FROM bucket_storage_tallies bst
                           INNER JOIN value_attributions va ON (
                      bst.project_id = va.project_id
                          AND bst.bucket_name = va.bucket_name
                      )
                  WHERE bst.interval_start >= @start
                    AND bst.interval_start < @end
                  GROUP BY va.user_agent,
                           bst.project_id,
                           bst.bucket_name,
                           timestamp_trunc(bst.interval_start, hour, "UTC")
                  ORDER BY max_interval DESC) bsti
                  INNER JOIN bucket_storage_tallies bsto ON (
             bsto.project_id = bsti.project_id
                 AND bsto.bucket_name = bsti.bucket_name
                 AND bsto.interval_start = bsti.max_interval
             )
         GROUP BY bsti.user_agent,
                  bsto.project_id,
                  bsto.bucket_name
         UNION ALL
         -- SUM the bandwidth for the timeframe specified grouping by the user_agent, project_id, and bucket_name
         SELECT va.user_agent   as user_agent,
                bbr.project_id  as project_id,
                bbr.bucket_name as bucket_name,
                0               as total,
                0               as remote,
                0               as inline,
                0               as segments,
                0               as objects,
                CAST(SUM(bbr.settled) as INT64) as settled, null as hours
         FROM bucket_bandwidth_rollups bbr
                  INNER JOIN value_attributions va ON (
             bbr.project_id = va.project_id
                 AND bbr.bucket_name = va.bucket_name
             )
         WHERE bbr.interval_start >= @start
           AND bbr.interval_start < @end
           -- action 2 is GET
           AND bbr.action = 2
         GROUP BY va.user_agent,
                  bbr.project_id,
                  bbr.bucket_name) AS o
GROUP BY o.user_agent,
         o.project_id,
         o.bucket_name;
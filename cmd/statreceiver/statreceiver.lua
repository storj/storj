-- possible sources:
--  * udpin(address)
--  * filein(path)
-- multiple sources can be handled in the same run (including multiple sources
-- of the same type) by calling deliver more than once.
source = udpin(":9000")

-- These two numbers are the size of destination metric buffers and packet buffers
-- respectively. Wrapping a metric destination in a metric buffer starts a goroutine
-- for that destination, which allows for concurrent writes to destinations, instead
-- of writing to destinations synchronously. If one destination blocks, this allows
-- the other destinations to continue, with the caveat that buffers may get overrun
-- if the buffer fills past this value.
-- One additional caveat to make sure mbuf and pbuf work - they need mbufprep and
-- pbufprep called higher in the pipeline. By default to save CPU cycles, memory
-- is reused, but this is bad in buffered writer situations. mbufprep and pbufprep
-- stress the garbage collector and lower performance at the expense of getting
-- mbuf and pbuf to work.
-- I've gone through and added mbuf and pbuf calls in various places. I think one
-- of our output destinations was getting behind and getting misconfigured, and
-- perhaps that was causing the holes in the data.
-- - JT 2019-05-15
mbufsize = 10000
pbufsize = 1000

-- multiple metric destination types
--  * graphite(address) goes to tcp with the graphite wire protocol
--  * print() goes to stdout
--  * db("sqlite3", path) goes to sqlite
--  * db("postgres", connstring) goes to postgres
   influx_out_old = graphite("influx-internal.datasci.storj.io.:2003")
   influx_out_v3 = influx("http://influx-internal.datasci.storj.io:8086/write?db=v3_stats_new")

v2_metric_handlers = sanitize(mbufprep(mbuf("influx_old", influx_out_old, mbufsize)))

--    mbuf(graphite_out_stefan, mbufsize),
  -- send specific storagenode data to the db
    --keyfilter(
      --"env\\.process\\." ..
        --"|hw\\.disk\\..*Used" ..
        --"|hw\\.disk\\..*Avail" ..
        --"|hw\\.network\\.stats\\..*\\.(tx|rx)_bytes\\.(deriv|val)",
      --mbuf(db_out, mbufsize))



v3_metric_handlers = mbufprep(mcopy(
    mbuf("downgrade", downgrade(v2_metric_handlers), mbufsize),
    mbuf("influx_new", influx_out_v3, mbufsize)
))

-- create a metric parser.
metric_parser =
  parse(  -- parse takes one or two arguments. the first argument is
          -- a metric handler, the remaining one is a per-packet application or
          -- instance filter. each filter is a regex. all packets must
          -- match all packet filters.
    versionsplit(v2_metric_handlers, v3_metric_handlers)) -- sanitize converts weird chars to underscores
    --packetfilter(".*", "", udpout("localhost:9002")))
    --packetfilter("(storagenode|satellite)-(dev|prod|alphastorj|stagingstorj)", ""))

af = "(satellite|downloadData|uploadData).*(-alpha|-release|storj|-transfersh)"
af_rothko = ".*(-alpha|-release|storj|-transfersh)"
uplink_header_matcher = headermultivalmatcher("sat",
    "12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@us-central-1.tardigrade.io:7777",
    "12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@mars.tardigrade.io:7777",
    "121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@asia-east-1.tardigrade.io:7777",
    "121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@saturn.tardigrade.io:7777",
    "12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@europe-west-1.tardigrade.io:7777",
    "12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@jupiter.tardigrade.io:7777",
    "118UWpMCHzs6CvSgWd9BfFVjw5K9pZbJjkfZJexMtSkmKxvvAW@satellite.stefan-benten.de:7777",
    "1wFTAgs9DP5RSnCqKV1eLf6N9wtk4EAtmN5DpSxcs8EjT69tGE@saltlake.tardigrade.io:7777")

-- pcopy forks data to multiple outputs
-- output types include parse, fileout, and udpout
destination = pbufprep(pcopy(
  --fileout("dump.out"),
  pbuf(packetfilter(af, "", nil, metric_parser), pbufsize),

  -- useful local debugging
  pbuf(udpout("localhost:9001"), pbufsize),

  -- rothko
   pbuf(packetfilter(af_rothko, "", nil, udpout("rothko-internal.datasci.storj.io:9002")), pbufsize)

   -- uplink
   --pbuf(packetfilter("uplink", "", uplink_header_matcher, packetprint()), pbufsize)
 ))

-- tie the source to the destination
deliver(source, destination)


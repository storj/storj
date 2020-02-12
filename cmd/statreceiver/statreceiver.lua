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
   influx_out = graphite("influx-internal.datasci.storj.io.:2003")
   graphite_out = graphite("graphite-internal.datasci.storj.io.:2003")

metric_handlers = sanitize(mbufprep(mcopy(
  -- send all satellite data to graphite
    mbuf(influx_out, mbufsize),
    mbuf(graphite_out, mbufsize))))
--    mbuf(graphite_out_stefan, mbufsize),
  -- send specific storagenode data to the db
    --keyfilter(
      --"env\\.process\\." ..
        --"|hw\\.disk\\..*Used" ..
        --"|hw\\.disk\\..*Avail" ..
        --"|hw\\.network\\.stats\\..*\\.(tx|rx)_bytes\\.(deriv|val)",
      --mbuf(db_out, mbufsize))



v3_metric_handlers = mcopy(
    downgrade(metric_handlers)
)

-- create a metric parser.
metric_parser =
  parse(  -- parse takes one or two arguments. the first argument is
          -- a metric handler, the remaining one is a per-packet application or
          -- instance filter. each filter is a regex. all packets must
          -- match all packet filters.
    versionsplit(metric_handlers, v3_metric_handlers)) -- sanitize converts weird chars to underscores
    --packetfilter(".*", "", udpout("localhost:9002")))
    --packetfilter("(storagenode|satellite)-(dev|prod|alphastorj|stagingstorj)", ""))

af = "(uplink|satellite|downloadData|uploadData).*(-alpha|-release|storj|-transfersh)"
af_rothko = ".*(-alpha|-release|storj|-transfersh)"

-- pcopy forks data to multiple outputs
-- output types include parse, fileout, and udpout
destination = pbufprep(pcopy(
  --fileout("dump.out"),
  pbuf(packetfilter(af, "", metric_parser), pbufsize),

  -- useful local debugging
  pbuf(udpout("localhost:9001"), pbufsize),

  -- rothko
   pbuf(packetfilter(af_rothko, "", udpout("rothko-internal.datasci.storj.io:9002")), pbufsize)
 ))

-- tie the source to the destination
deliver(source, destination)


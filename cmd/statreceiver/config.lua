-- possible sources:
--  * udpin(address)
--  * filein(path)
-- multiple sources can be handled in the same run (including multiple sources
-- of the same type) by calling deliver more than once.
source = udpin(":9000")

-- multiple metric destination types
--  * graphite(address) goes to tcp with the graphite wire protocol
--  * print() goes to stdout
--  * db("sqlite3", path) goes to sqlite
--  * db("postgres", connstring) goes to postgres
 graphite_out_local = graphite(graphite(os.getenv(GR_OUT_LOCAL)))
 graphite_out_remote1 = graphite(graphite(os.getenv(GR_OUT_REMOTE1)))
 graphite_out_remote2 = graphite(graphite(os.getenv(GR_OUT_REMOTE2)))
db_out = mcopy(
  db("sqlite3", "db.db"),
  db("postgres", "host=os.getenv(PG_HOST) port=os.getenv(PG_PORT) user=os.getenv(PG_USER) password=(PG_PASS) dbname=os.getenv(PG_DBNAME) sslmode=disable"))

metric_handlers = mcopy(
  -- send all satellite data to graphite
  appfilter(".*",
    graphite_out_local),
  appfilter(".*",
    graphite_out_remote1),
  appfilter(".*",
    graphite_out_remote2),
  -- send specific storagenode data to the db
  appfilter(".*",
    keyfilter(
      "env\\.process\\." ..
        "|hw\\.disk\\..*Used" ..
        "|hw\\.disk\\..*Avail" ..
        "|hw\\.network\\.stats\\..*\\.(tx|rx)_bytes\\.(deriv|val)",
      db_out)),
  -- just print uplink stuff
  appfilter("uplink-prod",
    print()))

-- create a metric parser.
metric_parser =
  parse(sanitize(metric_handlers)) -- sanitize converts weird chars to underscores
=======
  parse(  -- parse takes one or two arguments. the first argument is
          -- a metric handler, the remaining one is a per-packet application or
          -- instance filter. each filter is a regex. all packets must
          -- match all packet filters.
    sanitize(metric_handlers), -- sanitize converts weird chars to underscores
    packetfilter(".*", ""))

-- pcopy forks data to multiple outputs
-- output types include parse, fileout, packetfilter, and udpout
destination = pcopy(
  fileout("dump.out"),
  metric_parser,

  -- useful local debugging
  udpout("localhost:9001"),

  -- rothko
  packetfilter("storagenode-prod|satellite-prod|uplink-prod", "",
    udpout("localhost:9002")))

-- tie the source to the destination
deliver(source, destination)

-- possible sources:
--  * udpin(address)
--  * filein(path)
-- multiple sources can be handled in the same run (including multiple sources
-- of the same type) by calling deliver more than once.
source = udpin("localhost:9000")

-- multiple metric destination types
--  * graphite(address) goes to tcp with the graphite wire protocol
--  * print() goes to stdout
--  * db("sqlite3", path) goes to sqlite
--  * db("postgres", connstring) goes to postgres
graphite_out = graphite("localhost:5555")
db_out = mcopy(
  db("sqlite3", "db.db"),
  db("postgres", "user=dbuser dbname=dbname"))

metric_handlers = mcopy(
  -- send all satellite data to graphite
  appfilter("satellite-prod",
    graphite("localhost:5555")),
  -- send specific storagenode data to the db
  appfilter("storagenode-prod",
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

-- pcopy forks data to multiple outputs
-- output types include parse, fileout, and udpout
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

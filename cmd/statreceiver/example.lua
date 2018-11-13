deliver(
  udp("localhost:10001"),
  pcopy(
    fileout("dump.out"),
    parse(
      keyfilter(
        "env\\.process|hw\\.disk",
        sanitize(mcopy(
          print(),
          graphite("localhost:5555"),
          db("sqlite3", "db.db"),
          db("postgres", "user=dbuser dbname=dbname")))),
      appfilter("main-dev"))))

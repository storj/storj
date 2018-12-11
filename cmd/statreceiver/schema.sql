create table if not exists metrics (
  metric text,
  instance text,
  val real,
  timestamp integer,
  primary key (metric, instance)
);

create unique index metrics_index ON metrics(metric, instance);

alter table pathdata alter column bucket drop default;
alter table pathdata add constraint "pathdata_bucket_fkey" FOREIGN KEY (bucket) REFERENCES buckets(bucketname);

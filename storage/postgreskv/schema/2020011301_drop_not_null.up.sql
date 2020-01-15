alter table pathdata alter column bucket set default '';
alter table pathdata drop constraint pathdata_bucket_fkey;

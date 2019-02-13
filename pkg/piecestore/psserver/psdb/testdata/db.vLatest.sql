PRAGMA foreign_keys=OFF;
BEGIN TRANSACTION;
CREATE TABLE `ttl` (`id` BLOB UNIQUE, `created` INT(10), `expires` INT(10), `size` INT(10));
CREATE TABLE `bandwidth_agreements` (`satellite` BLOB, `agreement` BLOB, `signature` BLOB);
CREATE TABLE `bwusagetbl` (`size` INT(10), `daystartdate` INT(10), `dayenddate` INT(10));
CREATE TABLE sn_versions (version int, commited_at text);
INSERT INTO sn_versions VALUES(0,'1970-01-01 01:00:00 +0100 CET');
CREATE INDEX idx_ttl_expires ON ttl (expires);
COMMIT;

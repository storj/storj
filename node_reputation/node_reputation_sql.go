// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package nodereputation

var createStmt = `CREATE table node_reputation (
	source text not null,
	node_name text not null,
	timestamp timestamp DEFAULT(STRFTIME('%Y-%m-%d %H:%M:%f', 'NOW')) not null,
	uptime interger,
	audit_success interger,
	audit_fail interger,
	latency interger,
	amount_of_data_stored interger,
	false_claims interger,
	shards_modified interger,
PRIMARY KEY(node_name, timestamp)
);`

var insertStmt = `INSERT into node_reputation (
	source,
	node_name,
	uptime,
	audit_success,
	audit_fail,
	latency,
	amount_of_data_stored,
	false_claims,
	shards_modified
) values (?, ?, ?, ?, ?, ?, ?, ?, ?);`

var selectAllStmt = `SELECT
	source,
	node_name,
	timestamp,
	uptime,
	audit_success,
	audit_fail,
	latency,
	amount_of_data_stored,
	false_claims,
	shards_modified
FROM node_reputation`

var deletStmt = `DELETE FROM node_reputation
WHERE node_name = ?
AND timestamp NOT IN (
SELECT timestamp FROM node_reputation WHERE
source = ?
AND
node_name = ?
AND
timestamp = STRFTIME('%Y-%m-%d %H:%M:%f', ?)
AND
uptime = ?
AND
audit_success = ?
AND
audit_fail = ?
AND
latency = ?
AND
amount_of_data_stored = ?
AND
false_claims = ?
AND
shards_modified = ?)`

// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package nodereputation

import (
	"database/sql"
	"strconv"

	"golang.org/x/net/context"
	//"storj.io/storj/protos/node_reputation"
	proto "storj.io/storj/protos/node_reputation"
)

// Server is a struct
type Server struct{}

// UpdateReputation in handler
func (s *Server) UpdateReputation(ctx context.Context, in *proto.NodeUpdate) (*proto.UpdateReply, error) {
	db, err := SetServerDB("./Server.db")
	if err != nil {
		return nil, err
	}

	status := insertNodeUpdate(db, in)

	return &proto.UpdateReply{
		BridgeName: "Storj",
		NodeName:   in.NodeName,
		Status:     status,
	}, nil
}

// NodeReputation in handler
func (s *Server) NodeReputation(ctx context.Context, in *proto.NodeQuery) (*proto.NodeReputationRecord, error) {
	db, err := SetServerDB("./Server.db")
	if err != nil {
		return nil, err
	}
	node, err := byNodeName(db, in.NodeName)
	if err != nil {
		return nil, err
	}

	return &node, nil
}

// FilterNodeReputation in handler
func (s *Server) FilterNodeReputation(ctx context.Context, in *proto.NodeFilter) (*proto.NodeReputationRecords, error) {
	db, err := SetServerDB("./Server.db")
	if err != nil {
		return nil, err
	}
	nodes, err := selectNodeWhere(db, in.ColumnName, in.Operand, in.ColumnValue)
	if err != nil {
		return nil, err
	}

	return &nodes, nil
}

// PruneNodeReputation compresses a node's reputation
func (s *Server) PruneNodeReputation(ctx context.Context, in *proto.NodeQuery) (*proto.UpdateReply, error) {
	db, err := SetServerDB("./Server.db")
	if err != nil {
		return nil, err
	}

	status := deleteByNodeName(db, in.NodeName)

	return &proto.UpdateReply{
		BridgeName: "Storj",
		NodeName:   in.NodeName,
		Status:     status,
	}, nil

}

// byNodeName function used in handler by update reputation
func byNodeName(db *sql.DB, nodeName string) (proto.NodeReputationRecord, error) {
	var recordForError proto.NodeReputationRecord
	selectNodeStmt := genWhereStatement(selectAllStmt, nodeNameColumn, equal, nodeName)
	row, err := endianReputation(db, selectNodeStmt)
	if err != nil {
		return recordForError, err
	}

	return row.serde(row.naiveScore()), nil
}

// deleteByNodeName compresses a node's reputation
func deleteByNodeName(db *sql.DB, nodeName string) proto.UpdateReply_ReplyType {
	res := proto.UpdateReply_UPDATE_FAILED
	selectNodeStmt := genWhereStatement(selectAllStmt, nodeNameColumn, equal, nodeName)
	row, err := endianReputation(db, selectNodeStmt)
	if err != nil {
		res = proto.UpdateReply_UPDATE_FAILED
	}

	err = pruneNodeReputationRecords(db, row, deletStmt)
	if err != nil {
		res = proto.UpdateReply_UPDATE_FAILED
	}

	res = proto.UpdateReply_UPDATE_SUCCESS

	return res
}

// toWhereOpt is a method to convert a proto operand to a where operation
func toWhereOpt(opt proto.NodeFilter_Operand) whereOpt {
	res := notEqual
	switch opt {
	case proto.NodeFilter_EQUAL_TO:
		res = equal
	case proto.NodeFilter_GREATER_THAN:
		res = greater
	case proto.NodeFilter_GREATER_THAN_EQUAL_TO:
		res = greaterEqual
	case proto.NodeFilter_LESS_THAN:
		res = less
	case proto.NodeFilter_LESS_THAN_EQUAL_TO:
		res = lessEqual
	case proto.NodeFilter_NOT_EQUAL_TO:
		res = notEqual
	}
	return res
}

// toColum method converts a proto column type to a sum column type
func toColumn(col proto.ColumnName) column {
	res := sourceColumn
	switch col {
	case proto.ColumnName_source:
		res = sourceColumn
	case proto.ColumnName_node_name:
		res = nodeNameColumn
	case proto.ColumnName_timestamp:
		res = timestampColumn
	case proto.ColumnName_uptime:
		res = uptimeColumn
	case proto.ColumnName_audit_success:
		res = auditSuccessColumn
	case proto.ColumnName_audit_fail:
		res = auditFailColumn
	case proto.ColumnName_latency:
		res = latencyColumn
	case proto.ColumnName_amount_of_data_stored:
		res = amountOfDataStoredColumn
	case proto.ColumnName_false_claims:
		res = falseClaimsColumn
	case proto.ColumnName_shards_modified:
		res = shardsModifiedColumn
	}

	return res
}

// selectNodeWhere is a function that queries the reputation db and finds nodes that satisfies the where clause
func selectNodeWhere(db *sql.DB, col proto.ColumnName, operand proto.NodeFilter_Operand, value string) (proto.NodeReputationRecords, error) {
	var records []*proto.NodeReputationRecord
	recordsForError := proto.NodeReputationRecords{Records: records}

	selectNodeStmt := genWhereStatement(selectAllStmt, toColumn(col), toWhereOpt(operand), value)
	nodes, err := getNodeReputationRecords(db, selectNodeStmt)
	if err != nil {
		return recordsForError, err
	}

	for _, node := range nodes {
		n := node.serde(node.naiveScore())
		records = append(records, &n)
	}

	return proto.NodeReputationRecords{
		Records: records,
	}, nil
}

// insertNodeUpdate used in handler by update node reputation
func insertNodeUpdate(db *sql.DB, in *proto.NodeUpdate) proto.UpdateReply_ReplyType {
	res := proto.UpdateReply_UPDATE_FAILED

	selectNodeStmt := genWhereStatement(selectAllStmt, nodeNameColumn, equal, in.NodeName)
	row, err := endianReputation(db, selectNodeStmt)
	if err != nil {
		res = proto.UpdateReply_UPDATE_FAILED
	}

	if row.nodeName != in.NodeName {
		row.source = in.Source
		row.nodeName = in.NodeName
	}
	newRecord := row

	switch toColumn(in.ColumnName) {
	case uptimeColumn:
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = proto.UpdateReply_UPDATE_FAILED
		} else {
			res = proto.UpdateReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(uptimeColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)

	case auditSuccessColumn:
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = proto.UpdateReply_UPDATE_FAILED
		} else {
			res = proto.UpdateReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(auditSuccessColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)

	case auditFailColumn:
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = proto.UpdateReply_UPDATE_FAILED
		} else {
			res = proto.UpdateReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(auditFailColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)

	case latencyColumn:
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = proto.UpdateReply_UPDATE_FAILED
		} else {
			res = proto.UpdateReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(latencyColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)

	case amountOfDataStoredColumn:
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = proto.UpdateReply_UPDATE_FAILED
		} else {
			res = proto.UpdateReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(amountOfDataStoredColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)

	case falseClaimsColumn:
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = proto.UpdateReply_UPDATE_FAILED
		} else {
			res = proto.UpdateReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(falseClaimsColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)

	case shardsModifiedColumn:
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = proto.UpdateReply_UPDATE_FAILED
		} else {
			res = proto.UpdateReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(shardsModifiedColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)

	default:
		res = proto.UpdateReply_UPDATE_FAILED

	}

	return res
}

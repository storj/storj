// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package nodereputation

import (
	"database/sql"
	"strconv"

	"golang.org/x/net/context"
)

// Server is a struct
type Server struct{}

// UpdateReputation in handler
func (s *Server) UpdateReputation(ctx context.Context, in *NodeUpdate) (*UpdateReply, error) {
	db, err := SetServerDB("./Server.db")
	if err != nil {
		return nil, err
	}

	status := insertNodeUpdate(db, in)

	return &UpdateReply{
		BridgeName: "Storj",
		NodeName:   in.NodeName,
		Status:     status,
	}, nil
}

// NodeReputation in handler
func (s *Server) NodeReputation(ctx context.Context, in *NodeQuery) (*NodeReputationRecord, error) {
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
func (s *Server) FilterNodeReputation(ctx context.Context, in *NodeFilter) (*NodeReputationRecords, error) {
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
func (s *Server) PruneNodeReputation(ctx context.Context, in *NodeQuery) (*UpdateReply, error) {
	db, err := SetServerDB("./Server.db")
	if err != nil {
		return nil, err
	}

	status := deleteByNodeName(db, in.NodeName)

	return &UpdateReply{
		BridgeName: "Storj",
		NodeName:   in.NodeName,
		Status:     status,
	}, nil

}

// byNodeName function used in handler by update reputation
func byNodeName(db *sql.DB, nodeName string) (NodeReputationRecord, error) {
	var recordForError NodeReputationRecord
	selectNodeStmt := genWhereStatement(selectAllStmt, nodeNameColumn, equal, nodeName)
	row, err := endianReputation(db, selectNodeStmt)
	if err != nil {
		return recordForError, err
	}

	return row.serde(row.naiveScore()), nil
}

// deleteByNodeName compresses a node's reputation
func deleteByNodeName(db *sql.DB, nodeName string) UpdateReply_ReplyType {
	res := UpdateReply_UPDATE_FAILED
	selectNodeStmt := genWhereStatement(selectAllStmt, nodeNameColumn, equal, nodeName)
	row, err := endianReputation(db, selectNodeStmt)
	if err != nil {
		res = UpdateReply_UPDATE_FAILED
	}

	err = pruneNodeReputationRecords(db, row, deletStmt)
	if err != nil {
		res = UpdateReply_UPDATE_FAILED
	}

	res = UpdateReply_UPDATE_SUCCESS

	return res
}

// toWhereOpt is a method to convert a proto operand to a where operation
func (opt NodeFilter_Operand) toWhereOpt() whereOpt {
	res := notEqual
	switch opt {
	case NodeFilter_EQUAL_TO:
		res = equal
	case NodeFilter_GREATER_THAN:
		res = greater
	case NodeFilter_GREATER_THAN_EQUAL_TO:
		res = greaterEqual
	case NodeFilter_LESS_THAN:
		res = less
	case NodeFilter_LESS_THAN_EQUAL_TO:
		res = lessEqual
	case NodeFilter_NOT_EQUAL_TO:
		res = notEqual
	}
	return res
}

// toColum method converts a proto column type to a sum column type
func (col ColumnName) toColumn() column {
	res := sourceColumn
	switch col {
	case ColumnName_source:
		res = sourceColumn
	case ColumnName_node_name:
		res = nodeNameColumn
	case ColumnName_timestamp:
		res = timestampColumn
	case ColumnName_uptime:
		res = uptimeColumn
	case ColumnName_audit_success:
		res = auditSuccessColumn
	case ColumnName_audit_fail:
		res = auditFailColumn
	case ColumnName_latency:
		res = latencyColumn
	case ColumnName_amount_of_data_stored:
		res = amountOfDataStoredColumn
	case ColumnName_false_claims:
		res = falseClaimsColumn
	case ColumnName_shards_modified:
		res = shardsModifiedColumn
	}

	return res
}

// selectNodeWhere is a function that queries the reputation db and finds nodes that satisfies the where clause
func selectNodeWhere(db *sql.DB, col ColumnName, operand NodeFilter_Operand, value string) (NodeReputationRecords, error) {
	var records []*NodeReputationRecord
	recordsForError := NodeReputationRecords{Records: records}

	selectNodeStmt := genWhereStatement(selectAllStmt, col.toColumn(), operand.toWhereOpt(), value)
	nodes, err := getNodeReputationRecords(db, selectNodeStmt)
	if err != nil {
		return recordsForError, err
	}

	for _, node := range nodes {
		n := node.serde(node.naiveScore())
		records = append(records, &n)
	}

	return NodeReputationRecords{
		Records: records,
	}, nil
}

// insertNodeUpdate used in handler by update node reputation
func insertNodeUpdate(db *sql.DB, in *NodeUpdate) UpdateReply_ReplyType {
	res := UpdateReply_UPDATE_FAILED

	selectNodeStmt := genWhereStatement(selectAllStmt, nodeNameColumn, equal, in.NodeName)
	row, err := endianReputation(db, selectNodeStmt)
	if err != nil {
		res = UpdateReply_UPDATE_FAILED
	}

	if row.nodeName != in.NodeName {
		row.source = in.Source
		row.nodeName = in.NodeName
	}
	newRecord := row

	switch in.ColumnName.toColumn() {
	case uptimeColumn:
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = UpdateReply_UPDATE_FAILED
		} else {
			res = UpdateReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(uptimeColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)

	case auditSuccessColumn:
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = UpdateReply_UPDATE_FAILED
		} else {
			res = UpdateReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(auditSuccessColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)

	case auditFailColumn:
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = UpdateReply_UPDATE_FAILED
		} else {
			res = UpdateReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(auditFailColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)

	case latencyColumn:
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = UpdateReply_UPDATE_FAILED
		} else {
			res = UpdateReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(latencyColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)

	case amountOfDataStoredColumn:
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = UpdateReply_UPDATE_FAILED
		} else {
			res = UpdateReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(amountOfDataStoredColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)

	case falseClaimsColumn:
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = UpdateReply_UPDATE_FAILED
		} else {
			res = UpdateReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(falseClaimsColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)

	case shardsModifiedColumn:
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = UpdateReply_UPDATE_FAILED
		} else {
			res = UpdateReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(shardsModifiedColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)

	default:
		res = UpdateReply_UPDATE_FAILED

	}

	return res
}

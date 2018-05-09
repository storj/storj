package reputation

import (
	"database/sql"
	"strconv"
	"strings"

	"golang.org/x/net/context"
)

// Server is a struct
type Server struct{}

// UpdateReputation in handler
func (s *Server) UpdateReputation(ctx context.Context, in *NodeUpdate) (*BridgeReply, error) {
	db, err := SetServerDB("./Server.db")
	if err != nil {
		return nil, err
	}

	status := insertNodeUpdate(db, in)

	return &BridgeReply{
		BridgeName: "Storj",
		NodeName:   in.NodeName,
		Status:     status,
	}, nil
}

// QueryAggregatedNodeInfo in handler
func (s *Server) QueryAggregatedNodeInfo(ctx context.Context, in *NodeQuery) (*NodeReputation, error) {
	db, err := SetServerDB("./Server.db")
	if err != nil {
		return nil, err
	}
	nodes := byNodeName(db, in.NodeName)
	node := nodes[0]

	return &node, nil
}

func insertNodeUpdate(db *sql.DB, in *NodeUpdate) BridgeReply_ReplyType {
	res := BridgeReply_UPDATE_FAILED
	newRecord := newReputationRow(in.Source, in.NodeName)

	switch strings.ToLower(in.ColumnName) {
	case "uptime":
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = BridgeReply_UPDATE_FAILED
		} else {
			res = BridgeReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(uptimeColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)
	case "auditsuccess":
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = BridgeReply_UPDATE_FAILED
		} else {
			res = BridgeReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(auditSuccessColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)
	case "auditFail":
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = BridgeReply_UPDATE_FAILED
		} else {
			res = BridgeReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(auditFailColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)
	case "latency":
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = BridgeReply_UPDATE_FAILED
		} else {
			res = BridgeReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(latencyColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)
	case "amountofdatastored":
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = BridgeReply_UPDATE_FAILED
		} else {
			res = BridgeReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(amountOfDataStoredColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)
	case "falseclaims":
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = BridgeReply_UPDATE_FAILED
		} else {
			res = BridgeReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(falseClaimsColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)
	case "shardsmodified":
		val, err := strconv.ParseInt(in.ColumnValue, 10, 64)
		if err != nil {
			res = BridgeReply_UPDATE_FAILED
		} else {
			res = BridgeReply_UPDATE_SUCCESS
			newRecord = newRecord.morphism(shardsModifiedColumn, overWrite, val)
		}
		morph := []nodeReputationRecord{newRecord}
		insertRows(db, morph, insertStmt)

	default:
		res = 1

	}

	return res
}

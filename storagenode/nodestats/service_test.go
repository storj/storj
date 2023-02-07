// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package nodestats

import (
	"context"
	"reflect"
	"testing"

	"go.uber.org/zap"

	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/trust"
)

func TestService_GetReputationStats(t *testing.T) {
	type fields struct {
		log    *zap.Logger
		dialer rpc.Dialer
		trust  *trust.Pool
	}
	type args struct {
		ctx         context.Context
		satelliteID storj.NodeID
	}
	var tests []struct {
		name    string
		fields  fields
		args    args
		want    *reputation.Stats
		wantErr bool
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{
				log:    tt.fields.log,
				dialer: tt.fields.dialer,
				trust:  tt.fields.trust,
			}
			got, err := s.GetReputationStats(tt.args.ctx, tt.args.satelliteID)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.GetReputationStats() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Service.GetReputationStats() = %v, want %v", got, tt.want)
			}
		})
	}
}

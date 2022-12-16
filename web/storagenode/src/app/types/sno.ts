// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    BandwidthUsed,
    EgressUsed,
    IngressUsed,
    Node,
    SatelliteInfo,
    SatelliteScores,
    Stamp,
    Utilization,
} from '@/storagenode/sno/sno';

/**
 * Holds all node module state.
 */
export class StorageNodeState {
    public info: Node = new Node();
    public utilization: Utilization = new Utilization();
    public satellites: SatelliteInfo[] = [];
    public disqualifiedSatellites: SatelliteInfo[] = [];
    public suspendedSatellites: SatelliteInfo[] = [];
    public selectedSatellite: SatelliteInfo = new SatelliteInfo();
    public bandwidthChartData: BandwidthUsed[] = [];
    public egressChartData: EgressUsed[] = [];
    public ingressChartData: IngressUsed[] = [];
    public storageChartData: Stamp[] = [];
    public storageSummary = 0;
    public averageUsageBytes = 0;
    public bandwidthSummary = 0;
    public egressSummary = 0;
    public ingressSummary = 0;
    public satellitesScores: SatelliteScores[] = [];
    public audits: SatelliteScores = new SatelliteScores();
}

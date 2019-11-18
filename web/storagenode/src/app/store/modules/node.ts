// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { datesDiffInMinutes } from '@/app/utils/date';
import { SNOApi } from '@/storagenode/api/storagenode';
import { Dashboard, SatelliteInfo } from '@/storagenode/dashboard';
import { BandwidthUsed, EgressBandwidthUsed, IngressBandwidthUsed, Satellite, Stamp } from '@/storagenode/satellite';

export const NODE_MUTATIONS = {
    POPULATE_STORE: 'POPULATE_STORE',
    SELECT_SATELLITE: 'SELECT_SATELLITE',
};

export const NODE_ACTIONS = {
    GET_NODE_INFO: 'GET_NODE_INFO',
    SELECT_SATELLITE: 'SELECT_SATELLITE',
};

export const StatusOnline = 'Online';
export const StatusOffline = 'Offline';

const {
    POPULATE_STORE,
    SELECT_SATELLITE,
} = NODE_MUTATIONS;

const {
    GET_NODE_INFO,
} = NODE_ACTIONS;

const statusThreshHoldMinutes = 120;
const snoAPI = new SNOApi();

const allSatellites = {
    id: null,
    disqualified: null,
};

export const node = {
    state: {
        info: {
            id: '',
            status: StatusOffline,
            lastPinged: new Date(),
            startedAt: new Date(),
            version: '',
            wallet: '',
            isLastVersion: false
        },
        utilization: {
            bandwidth: {
                used: 0,
                remaining: 1,
                available: 1,
            },
            diskSpace: {
                used: 0,
                remaining: 1,
                available: 1,
            },
        },
        satellites: new Array<SatelliteInfo>(),
        disqualifiedSatellites: new Array<SatelliteInfo>(),
        selectedSatellite: allSatellites,
        bandwidthChartData: new Array<BandwidthUsed>(),
        egressBandwidthChartData: new Array<EgressBandwidthUsed>(),
        ingressBandwidthChartData: new Array<IngressBandwidthUsed>(),
        storageChartData: new Array<Stamp>(),
        storageSummary: 0,
        bandwidthSummary: 0,
        egressSummary: 0,
        ingressSummary: 0,
        checks: {
            uptime: 0,
            audit: 0,
        },
    },
    mutations: {
        [POPULATE_STORE](state: any, nodeInfo: Dashboard): void {
            state.info.id = nodeInfo.nodeID;
            state.info.isLastVersion = nodeInfo.isUpToDate;
            state.info.version = nodeInfo.version;
            state.info.wallet = nodeInfo.wallet;
            state.utilization.diskSpace.used = nodeInfo.diskSpace.used;
            state.utilization.diskSpace.remaining = nodeInfo.diskSpace.available - nodeInfo.diskSpace.used;
            state.utilization.diskSpace.available = nodeInfo.diskSpace.available;
            state.utilization.bandwidth.used = nodeInfo.bandwidth.used;
            state.utilization.bandwidth.remaining = nodeInfo.bandwidth.available - nodeInfo.bandwidth.used;
            state.utilization.bandwidth.available = nodeInfo.bandwidth.available;

            state.disqualifiedSatellites = nodeInfo.satellites.filter((satellite: SatelliteInfo) => {
                return satellite.disqualified;
            });

            state.satellites = nodeInfo.satellites ? nodeInfo.satellites : [];

            state.info.status = StatusOffline;

            state.info.startedAt = nodeInfo.startedAt;
            state.info.lastPinged = nodeInfo.lastPinged;

            if (datesDiffInMinutes(new Date(), new Date(nodeInfo.lastPinged)) < statusThreshHoldMinutes) {
                state.info.status = StatusOnline;
            }
        },
        [SELECT_SATELLITE](state: any, satelliteInfo: Satellite): void {
            if (satelliteInfo.id) {
                state.satellites.forEach(satellite => {
                    if (satelliteInfo.id === satellite.id) {
                        const audit = calculateSuccessRatio(
                            satelliteInfo.audit.successCount,
                            satelliteInfo.audit.totalCount
                        );

                        const uptime = calculateSuccessRatio(
                            satelliteInfo.uptime.successCount,
                            satelliteInfo.uptime.totalCount
                        );

                        state.selectedSatellite = satellite;
                        state.checks.audit = audit;
                        state.checks.uptime = uptime;
                    }

                    return;
                });
            }
            else {
                state.selectedSatellite = allSatellites;
            }

            state.bandwidthChartData = satelliteInfo.bandwidthDaily;
            state.egressBandwidthChartData = satelliteInfo.egressBandwidthDaily;
            state.ingressBandwidthChartData = satelliteInfo.ingressBandwidthDaily;
            state.storageChartData = satelliteInfo.storageDaily;
            state.bandwidthSummary = satelliteInfo.bandwidthSummary;
            state.egressSummary = satelliteInfo.egressSummary;
            state.ingressSummary = satelliteInfo.ingressSummary;
            state.storageSummary = satelliteInfo.storageSummary;
        },
    },
    actions: {
        [GET_NODE_INFO]: async function ({commit}: any): Promise<any> {
            const response = await snoAPI.dashboard();

            commit(NODE_MUTATIONS.POPULATE_STORE, response);
        },
        [NODE_ACTIONS.SELECT_SATELLITE]: async function ({commit}, id: any): Promise<any> {
            const response = id ? await snoAPI.satellite(id) : await snoAPI.satellites();

            commit(NODE_MUTATIONS.SELECT_SATELLITE, response);
        },
    },
};

/**
 * calculates percent of success attempts for reputation metric
 * @param successCount - holds amount of success attempts for reputation metric
 * @param totalCount - holds total amount of attempts for reputation metric
 */
function calculateSuccessRatio(successCount: number, totalCount: number) : number {
    if (totalCount === 0) {
        return 100;
    }

    return successCount / totalCount * 100;
}

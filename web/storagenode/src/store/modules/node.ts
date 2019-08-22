// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { NODE_ACTIONS, NODE_MUTATIONS } from '@/utils/constants';
import { httpGet } from '@/api/storagenode';
import { ChartFormatter } from '@/utils/chartModule';

export const StatusOnline = 'Online';
export const StatusOffline = 'Offline';

const statusThreshHoldMinutes = 10;

const allSatellites = {
    id: null,
    disqualified: null,
};

export const node = {
    state: {
        info: {
            id: '',
            status: StatusOffline,
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
        satellites: [],
        disqualifiedSatellites: [],
        selectedSatellite: allSatellites,
        bandwidthChartData: ChartFormatter.createBandwidthChartItems([]),
        storageChartData: ChartFormatter.createStorageUsageChartItems([]),
        storageSummary: 0,
        bandwidthSummary: 0,
        checks: {
            uptime: 0,
            audit: 0,
        },
    },
    mutations: {
        [NODE_MUTATIONS.POPULATE_STORE](state: any, nodeInfo: any): void {
            state.info.id = nodeInfo.nodeID;
            state.info.isLastVersion = nodeInfo.upToDate;
            state.info.version = nodeInfo.version;
            state.info.wallet = nodeInfo.wallet;
            state.utilization.diskSpace.used = nodeInfo.diskSpace.used;
            state.utilization.diskSpace.remaining = nodeInfo.diskSpace.available - nodeInfo.diskSpace.used;
            state.utilization.diskSpace.available = nodeInfo.diskSpace.available;
            state.utilization.bandwidth.used = nodeInfo.bandwidth.used;
            state.utilization.bandwidth.remaining = nodeInfo.bandwidth.available - nodeInfo.bandwidth.used;
            state.utilization.bandwidth.available = nodeInfo.bandwidth.available;
            state.disqualifiedSatellites = [];

            state.satellites = nodeInfo.satellites ? nodeInfo.satellites.map(elem => {
                let satellite = {
                    id: elem.id,
                    disqualified: elem.disqualified ? new Date(elem.disqualified) : null,
                };

                if (satellite.disqualified) {
                    state.disqualifiedSatellites.push(satellite);
                }

                return {
                    id: elem.id,
                    disqualified: elem.disqualified ? new Date(elem.disqualified) : null,
                };
            }) : null;

            state.info.status = StatusOffline;
            if (getDateDiffMinutes(new Date(), new Date(nodeInfo.lastPinged)) < statusThreshHoldMinutes) {
                state.info.status = StatusOnline;
            }
        },
        [NODE_MUTATIONS.SELECT_SATELLITE](state: any, satelliteInfo: any): void {
            if (satelliteInfo.id) {
                state.satellites.forEach(satellite => {
                    if (satelliteInfo.id === satellite.id) {
                        const audit = calculateSuccessRatio(
                            satelliteInfo.audit.successCount,
                            satelliteInfo.audit.totalCount);

                        const uptime = calculateSuccessRatio(
                            satelliteInfo.uptime.successCount,
                            satelliteInfo.uptime.totalCount);

                        state.selectedSatellite = satellite;
                        state.checks.audit = audit;
                        state.checks.uptime = uptime;
                    }
                });
            }
            else {
                state.selectedSatellite = allSatellites;
            }

            state.bandwidthChartData = ChartFormatter.createBandwidthChartItems(satelliteInfo.bandwidthDaily);
            state.storageChartData = ChartFormatter.createStorageUsageChartItems(satelliteInfo.storageDaily);

            state.bandwidthSummary = satelliteInfo.bandwidthSummary;
            state.storageSummary = satelliteInfo.storageSummary;

            console.log("storage chart data: ", state.storageChartData);
            console.log("bandwidth chart data: ", state.bandwidthChartData);
        },
    },
    actions: {
        [NODE_ACTIONS.GET_NODE_INFO]: async function ({commit}: any): Promise<any> {
            const url = '/api/dashboard';

            let response = await httpGet(url);
            if (response.data) {
                commit(NODE_MUTATIONS.POPULATE_STORE, response.data);
                console.log("Response: ", response);
                return;
            }

            console.error('Error while fetching Node info!');
        },
        [NODE_ACTIONS.SELECT_SATELLITE]: async function ({commit}, id: any): Promise<any> {
            const url = id ? '/api/satellite/' + id : "/api/satellites";

            let response = await httpGet(url);
            if (response.data) {
                commit(NODE_MUTATIONS.SELECT_SATELLITE, response.data);
                console.log("Response: ", response);
                return;
            }

            console.error('Error while fetching Node info!');
        },
    },
};

// calculateSuccessRatio calculates percent of success attempts for reputation metric
function calculateSuccessRatio(successCount: number, totalCount: number) : number {
    if (totalCount == 0) {
        return 100;
    }

    return successCount / totalCount * 100;
}

// getDateDiffMinutes returns difference between two dates in minutes
function getDateDiffMinutes(d1: Date, d2: Date): number {
    const diff = d1.getTime() - d2.getTime();
    return Math.floor(diff/1000/60);
}
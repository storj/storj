// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { Duration, millisecondsInSecond, secondsInMinute } from '@/app/utils/duration';
import { getMonthsBeforeNow } from '@/app/utils/payout';
import { SNOApi } from '@/storagenode/api/storagenode';
import { Dashboard, SatelliteInfo } from '@/storagenode/dashboard';
import { BandwidthUsed, EgressUsed, IngressUsed, Satellite, Stamp } from '@/storagenode/satellite';

export const NODE_MUTATIONS = {
    POPULATE_STORE: 'POPULATE_STORE',
    SELECT_SATELLITE: 'SELECT_SATELLITE',
    SELECT_ALL_SATELLITES: 'SELECT_ALL_SATELLITES',
    SET_DAILY_DATA: 'SET_DAILY_DATA',
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
    SELECT_ALL_SATELLITES,
    SET_DAILY_DATA,
} = NODE_MUTATIONS;

const statusThreshHoldMinutes = 120;

export function makeNodeModule(api: SNOApi) {
    return {
        state: {
            info: {
                id: '',
                status: StatusOffline,
                lastPinged: new Date(),
                startedAt: new Date(),
                version: '',
                allowedVersion: '',
                wallet: '',
                isLastVersion: false,
            },
            utilization: {
                bandwidth: {
                    used: 0,
                },
                diskSpace: {
                    used: 0,
                    available: 1,
                    trash: 0,
                },
            },
            satellites: new Array<SatelliteInfo>(),
            disqualifiedSatellites: new Array<SatelliteInfo>(),
            suspendedSatellites: new Array<SatelliteInfo>(),
            selectedSatellite: {
                id: null,
                disqualified: null,
                joinDate: new Date(),
            },
            bandwidthChartData: new Array<BandwidthUsed>(),
            egressChartData: new Array<EgressUsed>(),
            ingressChartData: new Array<IngressUsed>(),
            storageChartData: new Array<Stamp>(),
            storageSummary: 0,
            bandwidthSummary: 0,
            egressSummary: 0,
            ingressSummary: 0,
            satellitesScores: new Array<SatelliteScores>(),
            checks: {
                uptime: 0,
                audit: 0,
                suspension: 0,
            },
        },
        satellites: new Array<SatelliteInfo>(),
        disqualifiedSatellites: new Array<SatelliteInfo>(),
        selectedSatellite: allSatellites,
        bandwidthChartData: new Array<BandwidthUsed>(),
        egressChartData: new Array<EgressUsed>(),
        ingressChartData: new Array<IngressUsed>(),
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

                const minutesPassed = Duration.difference(new Date(), new Date(nodeInfo.lastPinged)) / millisecondsInSecond / secondsInMinute;

                state.info.status = minutesPassed < statusThreshHoldMinutes ? StatusOnline : StatusOffline;
            },
            [SELECT_SATELLITE](state: any, satelliteInfo: Satellite): void {
                const selectedSatellite = state.satellites.find(satellite => satelliteInfo.id === satellite.id);

                if (!selectedSatellite) {
                    return;
                });
            }
            else {
                state.selectedSatellite = allSatellites;
            }

            state.bandwidthChartData = satelliteInfo.bandwidthDaily;
            state.egressChartData = satelliteInfo.egressDaily;
            state.ingressChartData = satelliteInfo.ingressDaily;
            state.storageChartData = satelliteInfo.storageDaily;
            state.bandwidthSummary = satelliteInfo.bandwidthSummary;
            state.egressSummary = satelliteInfo.egressSummary;
            state.ingressSummary = satelliteInfo.ingressSummary;
            state.storageSummary = satelliteInfo.storageSummary;
        },
        actions: {
            [NODE_ACTIONS.GET_NODE_INFO]: async function ({commit}: any): Promise<void> {
                const response = await api.dashboard();

                commit(NODE_MUTATIONS.POPULATE_STORE, response);
            },
            [NODE_ACTIONS.SELECT_SATELLITE]: async function ({commit}, id?: string): Promise<void> {
                let response: Satellite | Satellites;
                if (id) {
                    response = await api.satellite(id);
                    commit(NODE_MUTATIONS.SELECT_SATELLITE, response);
                } else {
                    response = await api.satellites();
                    commit(NODE_MUTATIONS.SELECT_ALL_SATELLITES, response);
                }

                commit(NODE_MUTATIONS.SET_DAILY_DATA, response);
            },
        },
        getters: {
            monthsOnNetwork: (state): number => {
                return getMonthsBeforeNow(state.selectedSatellite.joinDate);
            },
        },
    };
}

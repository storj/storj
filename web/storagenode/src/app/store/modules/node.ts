// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/app/store';
import { StorageNodeState } from '@/app/types/sno';
import { Duration, millisecondsInSecond, secondsInMinute } from '@/app/utils/duration';
import { getMonthsBeforeNow } from '@/app/utils/payout';
import { StorageNodeService } from '@/storagenode/sno/service';
import {
    Checks,
    Dashboard,
    Node,
    Satellite,
    SatelliteInfo,
    Satellites,
    Traffic,
    Utilization,
} from '@/storagenode/sno/sno';

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

const STATUS_TRESHHOLD_MINUTES: number = 120;

export function newNodeModule(service: StorageNodeService): StoreModule<StorageNodeState> {
    return {
        state: new StorageNodeState(),
        mutations: {
            [POPULATE_STORE](state: any, nodeInfo: Dashboard): void {
                const minutesPassed = Duration.difference(new Date(), new Date(nodeInfo.lastPinged)) / millisecondsInSecond / secondsInMinute;
                const status = minutesPassed < STATUS_TRESHHOLD_MINUTES ? StatusOnline : StatusOffline;
                state.info = new Node(
                    nodeInfo.nodeID,
                    status,
                    nodeInfo.lastPinged,
                    nodeInfo.startedAt,
                    nodeInfo.version,
                    nodeInfo.allowedVersion,
                    nodeInfo.wallet,
                    nodeInfo.isUpToDate,
                );

                state.utilization = new Utilization(
                    new Traffic(nodeInfo.bandwidth.used),
                    new Traffic(nodeInfo.diskSpace.used, nodeInfo.diskSpace.available, nodeInfo.diskSpace.trash),
                );

                state.disqualifiedSatellites = nodeInfo.satellites.filter((satellite: SatelliteInfo) => satellite.disqualified);
                state.suspendedSatellites = nodeInfo.satellites.filter((satellite: SatelliteInfo) => satellite.suspended);

                state.satellites = nodeInfo.satellites;
            },
            [SELECT_SATELLITE](state: any, satelliteInfo: Satellite): void {
                const selectedSatellite = state.satellites.find(satellite => satelliteInfo.id === satellite.id);

                if (!selectedSatellite) {
                    return;
                }

                state.selectedSatellite = new SatelliteInfo(
                    satelliteInfo.id,
                    selectedSatellite.url,
                    selectedSatellite.disqualified,
                    selectedSatellite.suspended,
                    satelliteInfo.joinDate,
                );

                state.checks = new Checks(satelliteInfo.uptime, satelliteInfo.audit);
            },
            [SELECT_ALL_SATELLITES](state: any, satelliteInfo: Satellites): void {
                state.selectedSatellite = new SatelliteInfo(
                    '',
                    '',
                    null,
                    null,
                    satelliteInfo.joinDate,
                );
                state.satellitesScores = satelliteInfo.satellitesScores;
            },
            [SET_DAILY_DATA](state: any, satelliteInfo: Satellite): void {
                state.bandwidthChartData = satelliteInfo.bandwidthDaily;
                state.egressChartData = satelliteInfo.egressDaily;
                state.ingressChartData = satelliteInfo.ingressDaily;
                state.storageChartData = satelliteInfo.storageDaily;
                state.bandwidthSummary = satelliteInfo.bandwidthSummary;
                state.egressSummary = satelliteInfo.egressSummary;
                state.ingressSummary = satelliteInfo.ingressSummary;
                state.storageSummary = satelliteInfo.storageSummary;
            },
        },
        actions: {
            [NODE_ACTIONS.GET_NODE_INFO]: async function ({commit}: any): Promise<void> {
                const dashboard = await service.dashboard();

                commit(NODE_MUTATIONS.POPULATE_STORE, dashboard);
            },
            [NODE_ACTIONS.SELECT_SATELLITE]: async function ({commit}, id?: string): Promise<void> {
                let response: Satellite | Satellites;
                if (id) {
                    response = await service.satellite(id);
                    commit(NODE_MUTATIONS.SELECT_SATELLITE, response);
                } else {
                    response = await service.satellites();
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

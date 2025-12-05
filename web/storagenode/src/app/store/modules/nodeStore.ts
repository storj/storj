// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { reactive } from 'vue';

import { StorageNodeState } from '@/app/types/sno';
import { Duration, millisecondsInSecond, secondsInMinute } from '@/app/utils/duration';
import { StorageNodeService } from '@/storagenode/sno/service';
import {
    Node,
    Satellite,
    SatelliteInfo,
    Satellites,
    Utilization,
} from '@/storagenode/sno/sno';
import { StorageNodeApi } from '@/storagenode/api/storagenode';

export const StatusOnline = 'Online';
export const StatusOffline = 'Offline';

export const QUIC_STATUS = {
    StatusOk: 'OK',
    StatusMisconfigured: 'Misconfigured',
    StatusRefreshing: 'Refreshing',
};

const STATUS_TRESHHOLD_MINUTES = 120;

export const useNodeStore = defineStore('nodeStore', () => {
    const state = reactive<StorageNodeState>(new StorageNodeState());

    const service = new StorageNodeService(new StorageNodeApi());

    async function fetchNodeInfo(): Promise<void> {
        const dashboard = await service.dashboard();

        const minutesPassed = Duration.difference(new Date(), new Date(dashboard.lastPinged)) / millisecondsInSecond / secondsInMinute;
        const status = minutesPassed < STATUS_TRESHHOLD_MINUTES ? StatusOnline : StatusOffline;
        state.info = new Node(
            dashboard.nodeID,
            status,
            dashboard.lastPinged,
            dashboard.startedAt,
            dashboard.version,
            dashboard.allowedVersion,
            dashboard.wallet,
            dashboard.walletFeatures,
            dashboard.isUpToDate,
            dashboard.quicStatus,
            dashboard.configuredPort,
            dashboard.lastQuicPingedAt,
        );

        state.utilization = new Utilization(
            dashboard.bandwidth,
            dashboard.diskSpace,
        );

        state.disqualifiedSatellites = dashboard.satellites.filter((satellite: SatelliteInfo) => satellite.disqualified);
        state.suspendedSatellites = dashboard.satellites.filter((satellite: SatelliteInfo) => satellite.suspended);

        state.satellites = dashboard.satellites;
    }

    async function selectSatellite(id?: string): Promise<void> {
        const response: Satellite | Satellites = await (id ? service.satellite(id) : service.satellites());

        if (response instanceof Satellite) {
            const sel = state.satellites.find(s => s.id === response.id);
            if (!sel) return;

            state.selectedSatellite = new SatelliteInfo(
                response.id,
                sel.url,
                sel.disqualified,
                sel.suspended,
                sel.vettedAt,
                response.joinDate,
            );

            state.audits = response.audits;
        } else {
            state.selectedSatellite = new SatelliteInfo(
                '',
                '',
                null,
                null,
                null,
                response.joinDate,
            );
            state.satellitesScores = response.satellitesScores;
        }

        state.bandwidthChartData = response.bandwidthDaily;
        state.egressChartData = response.egressDaily;
        state.ingressChartData = response.ingressDaily;
        state.storageChartData = response.storageDaily;
        state.bandwidthSummary = response.bandwidthSummary;
        state.egressSummary = response.egressSummary;
        state.ingressSummary = response.ingressSummary;
        state.storageSummary = response.storageSummary;
        state.averageUsageBytes = response.averageUsageBytes;
    }

    return {
        state,
        fetchNodeInfo,
        selectSatellite,
    };
});

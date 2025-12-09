// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { reactive } from 'vue';

import { BandwidthTraffic } from '@/bandwidth';
import { Bandwidth } from '@/bandwidth/service';
import { BandwidthClient } from '@/api/bandwidth';
import { useNodesStore } from '@/app/store/nodesStore';

class BandwidthState {
    public traffic: BandwidthTraffic = new BandwidthTraffic();
}

export const useBandwidthStore = defineStore('bandwidth', () => {
    const state = reactive<BandwidthState>(new BandwidthState());

    const service = new Bandwidth(new BandwidthClient());

    const nodesStore = useNodesStore();

    async function fetch(): Promise<void> {
        const selectedSatelliteId = nodesStore.state.selectedSatellite ? nodesStore.state.selectedSatellite.id : null;
        const selectedNodeId = nodesStore.state.selectedNode ? nodesStore.state.selectedNode.id : null;

        state.traffic = await service.fetch(selectedSatelliteId, selectedNodeId);
    }

    return {
        state,
        fetch,
    };
});
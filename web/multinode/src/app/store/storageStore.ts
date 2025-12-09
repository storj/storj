// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { reactive } from 'vue';

import { DiskSpace, DiskSpaceUsage } from '@/storage';
import { StorageService } from '@/storage/service';
import { StorageClient } from '@/api/storage';
import { useNodesStore } from '@/app/store/nodesStore';

class StorageState {
    public usage: DiskSpaceUsage = new DiskSpaceUsage();
    public diskSpace: DiskSpace = new DiskSpace();
}

export const useStorageStore = defineStore('storage', () => {
    const state = reactive(new StorageState());

    const service = new StorageService(new StorageClient());

    const nodesStore = useNodesStore();

    async function usage(): Promise<void> {
        const selectedSatelliteID = nodesStore.state.selectedSatellite ? nodesStore.state.selectedSatellite.id : null;
        const selectedNodeID = nodesStore.state.selectedNode ? nodesStore.state.selectedNode.id : null;

        state.usage = await service.usage(selectedSatelliteID, selectedNodeID);
    }

    async function diskSpace(): Promise<void> {
        const selectedNodeID = nodesStore.state.selectedNode ? nodesStore.state.selectedNode.id : null;

        state.diskSpace = await service.diskSpace(selectedNodeID);
    }

    return {
        state,
        usage,
        diskSpace,
    };
});

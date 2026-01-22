// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { reactive } from 'vue';

import { NodeFullInfo, NodeManagementHttpApiV1 } from '@/api/client.gen';

class NodesState {}

export const useNodesStore = defineStore('nodes', () => {
    const state = reactive<NodesState>(new NodesState());

    const nodesApi = new NodeManagementHttpApiV1();

    async function getNodeById(nodeId: string): Promise<NodeFullInfo> {
        return await nodesApi.getNodeInfo(nodeId);
    }

    return {
        state,
        getNodeById,
    };
});

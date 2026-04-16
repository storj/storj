// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { reactive } from 'vue';

import { DisqualifyNodeRequest, NodeFullInfo, NodeManagementHttpApiV1 } from '@/api/client.gen';

class NodesState {}

export const useNodesStore = defineStore('nodes', () => {
    const state = reactive<NodesState>(new NodesState());

    const nodesApi = new NodeManagementHttpApiV1();

    async function getNodeById(nodeId: string): Promise<NodeFullInfo> {
        return await nodesApi.getNodeInfo(nodeId);
    }

    async function disqualifyNode(nodeId: string, reason: string, disqualificationReason: string): Promise<void> {
        const request = new DisqualifyNodeRequest();
        request.reason = reason;
        request.disqualificationReason = disqualificationReason;
        return await nodesApi.disqualifyNode(request, nodeId);
    }

    return {
        state,
        getNodeById,
        disqualifyNode,
    };
});

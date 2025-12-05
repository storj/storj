// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { reactive } from 'vue';

import { CreateNodeFields, Node, NodeStatus, NodeURL, UpdateNodeModel } from '@/nodes';
import { Nodes } from '@/nodes/service';
import { NodesClient } from '@/api/nodes';

class NodesState {
    public nodes: Node[] = [];
    public selectedSatellite: NodeURL | null = null;
    public selectedNode: Node | null = null;
    public trustedSatellites: NodeURL[] = [];
}

export const useNodesStore = defineStore('nodes', () => {
    const state = reactive<NodesState>(new NodesState());

    const service = new Nodes(new NodesClient());

    async function fetch(): Promise<void> {
        state.nodes = state.selectedSatellite ? await service.listBySatellite(state.selectedSatellite.id) : await service.list();
    }

    async function fetchOnline(): Promise<void> {
        const nodes = state.selectedSatellite ? await service.listBySatellite(state.selectedSatellite.id) : await service.list();
        let onlineNodes;
        if (Array.isArray(nodes)) {
            onlineNodes = nodes.filter((node: Node) => node.status === NodeStatus['online']);
        } else {
            onlineNodes = nodes;
        }

        state.nodes = onlineNodes;
    }

    async function add(node: CreateNodeFields): Promise<void> {
        await service.add(node);
        await fetch();
    }

    async function deleteNode(nodeID: string): Promise<void> {
        await service.delete(nodeID);
        await fetch();
    }

    async function updateName(node: UpdateNodeModel): Promise<void> {
        await service.updateName(node.id, node.name);
        await fetch();
    }

    async function trustedSatellites(): Promise<void> {
        state.trustedSatellites = await service.trustedSatellites();
    }

    async function selectSatellite(satelliteID: string): Promise<void> {
        await trustedSatellites();

        state.selectedSatellite = state.trustedSatellites.find((satellite: NodeURL) => satellite.id === satelliteID) || null;

        await fetchOnline();
    }

    async function selectNode(nodeID: string | null): Promise<void> {
        state.selectedNode = state.nodes.find((node: Node) => node.id === nodeID) || null;

        await fetchOnline();
    }

    return {
        state,
        fetch,
        fetchOnline,
        add,
        deleteNode,
        updateName,
        trustedSatellites,
        selectSatellite,
        selectNode,
    };
});

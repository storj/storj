// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { ActionContext, ActionTree, GetterTree, Module, MutationTree } from 'vuex';

import { RootState } from '@/app/store/index';
import { DiskSpace, DiskSpaceUsage } from '@/storage';
import { StorageService } from '@/storage/service';

/**
 * StorageState is a representation of by day and total storage usage.
 */
export class StorageState {
    public usage: DiskSpaceUsage = new DiskSpaceUsage();
    public diskSpace: DiskSpace = new DiskSpace();
}

/**
 * StorageModule is a part of a global store that encapsulates all storage related logic.
 */
export class StorageModule implements Module<StorageState, RootState> {
    public readonly namespaced: boolean;
    public readonly state: StorageState;
    public readonly getters?: GetterTree<StorageState, RootState>;
    public readonly actions: ActionTree<StorageState, RootState>;
    public readonly mutations: MutationTree<StorageState>;

    private readonly storage: StorageService;

    public constructor(storage: StorageService) {
        this.storage = storage;

        this.namespaced = true;
        this.state = new StorageState();

        this.mutations = {
            setUsage: this.setUsage,
            setDiskSpace: this.setDiskSpace,
        };

        this.actions = {
            usage: this.usage.bind(this),
            diskSpace: this.diskSpace.bind(this),
        };
    }

    /**
     * setUsage mutation will set storage usage.
     * @param state - state of the module.
     * @param usage
     */
    public setUsage(state: StorageState, usage: DiskSpaceUsage): void {
        state.usage = usage;
    }

    /**
     * setDiskSpace mutation will set storage totals.
     * @param state - state of the module.
     * @param diskSpace
     */
    public setDiskSpace(state: StorageState, diskSpace: DiskSpace): void {
        state.diskSpace = diskSpace;
    }

    /**
     * usage action loads storage usage information.
     * @param ctx - context of the Vuex action.
     */
    public async usage(ctx: ActionContext<StorageState, RootState>): Promise<void> {
        const selectedSatelliteId = ctx.rootState.nodes.selectedSatellite ? ctx.rootState.nodes.selectedSatellite.id : ctx.rootState.nodes.trustedSatellites[0].id;
        const selectedNodeId = ctx.rootState.nodes.selectedNode ? ctx.rootState.nodes.selectedNode.id : null;

        const usage = await this.storage.usage(selectedSatelliteId, selectedNodeId);

        ctx.commit('setUsage', usage);
    }

    /**
     * diskSpace action loads total storage usage information.
     * @param ctx - context of the Vuex action.
     */
    public async diskSpace(ctx: ActionContext<StorageState, RootState>): Promise<void> {
        const selectedNodeId = ctx.rootState.nodes.selectedNode ? ctx.rootState.nodes.selectedNode.id : null;

        const diskSpace = await this.storage.diskSpace(selectedNodeId);

        ctx.commit('setDiskSpace', diskSpace);
    }
}

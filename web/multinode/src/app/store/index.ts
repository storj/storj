// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex, { ModuleTree, Store, StoreOptions } from 'vuex';

import { BandwidthClient } from '@/api/bandwidth';
import { NodesClient } from '@/api/nodes';
import { Operators as OperatorsClient } from '@/api/operators';
import { PayoutsClient } from '@/api/payouts';
import { BandwidthModule, BandwidthState } from '@/app/store/bandwidth';
import { NodesModule, NodesState } from '@/app/store/nodes';
import { OperatorsModule, OperatorsState } from '@/app/store/operators';
import { PayoutsModule, PayoutsState } from '@/app/store/payouts';
import { Bandwidth } from '@/bandwidth/service';
import { Nodes } from '@/nodes/service';
import { Operators } from '@/operators';
import { Payouts } from '@/payouts/service';

Vue.use(Vuex);

/**
 * RootState is a representation of global state.
 */
export class RootState {
    nodes: NodesState;
    payouts: PayoutsState;
    operators: OperatorsState;
    bandwidth: BandwidthState;
}

/**
 * MultinodeStoreOptions contains all needed data for store creation.
 */
export class MultinodeStoreOptions implements StoreOptions<RootState> {
    public readonly strict: boolean;
    public readonly state: RootState;
    public readonly modules: ModuleTree<RootState>;

    public constructor(
        nodes: NodesModule,
        payouts: PayoutsModule,
        operators: OperatorsModule,
        bandwidth: BandwidthModule,
    ) {
        this.strict = true;
        this.state = {
            nodes: nodes.state,
            payouts: payouts.state,
            bandwidth: bandwidth.state,
            operators: operators.state,
        };
        this.modules = {
            nodes,
            payouts,
            bandwidth,
            operators,
        };
    }
}

// Services
const nodesClient: NodesClient = new NodesClient();
const nodesService: Nodes = new Nodes(nodesClient);
const payoutsClient: PayoutsClient = new PayoutsClient();
const payoutsService: Payouts = new Payouts(payoutsClient);
const bandwidthClient = new BandwidthClient();
const bandwidthService = new Bandwidth(bandwidthClient);
const operatorsClient: OperatorsClient = new OperatorsClient();
const operatorsService: Operators = new Operators(operatorsClient);

// Modules
const nodesModule: NodesModule = new NodesModule(nodesService);
const payoutsModule: PayoutsModule = new PayoutsModule(payoutsService);
const bandwidthModule: BandwidthModule = new BandwidthModule(bandwidthService);
const operatorsModule: OperatorsModule = new OperatorsModule(operatorsService);

// Store
export const store: Store<RootState> = new Vuex.Store<RootState>(
    new MultinodeStoreOptions(nodesModule, payoutsModule, operatorsModule, bandwidthModule),
);

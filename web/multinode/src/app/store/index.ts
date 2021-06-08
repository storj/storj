// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex, { ModuleTree, Store, StoreOptions } from 'vuex';

import { NodesClient } from '@/api/nodes';
import { Operators as OperatorsClient } from '@/api/operators';
import { PayoutsClient } from '@/api/payouts';
import { NodesModule, NodesState } from '@/app/store/nodes';
import { OperatorsModule, OperatorsState } from '@/app/store/operators';
import { PayoutsModule, PayoutsState } from '@/app/store/payouts';
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
}

/**
 * MultinodeStoreOptions contains all needed data for store creation.
 */
export class MultinodeStoreOptions implements StoreOptions<RootState> {
    public readonly strict: boolean;
    public readonly state: RootState;
    public readonly modules: ModuleTree<RootState>;

    public constructor(nodes: NodesModule, payouts: PayoutsModule, operators: OperatorsModule) {
        this.strict = true;
        this.state = {
            nodes: nodes.state,
            payouts: payouts.state,
            operators: operators.state,
        };
        this.modules = {
            nodes,
            payouts,
            operators,
        };
    }
}

// Services
const nodesClient: NodesClient = new NodesClient();
const nodesService: Nodes = new Nodes(nodesClient);
const payoutsClient: PayoutsClient = new PayoutsClient();
const payoutsService: Payouts = new Payouts(payoutsClient);
const operatorsClient: OperatorsClient = new OperatorsClient();
const operatorsService: Operators = new Operators(operatorsClient);

// Modules
const nodesModule: NodesModule = new NodesModule(nodesService);
const payoutsModule: PayoutsModule = new PayoutsModule(payoutsService);
const operatorsModule: OperatorsModule = new OperatorsModule(operatorsService);

// Store
export const store: Store<RootState> = new Vuex.Store<RootState>(
    new MultinodeStoreOptions(nodesModule, payoutsModule, operatorsModule),
);

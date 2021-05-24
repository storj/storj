// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import { NodesClient } from '@/api/nodes';
import { PayoutsClient } from '@/api/payouts';
import { NodesModule } from '@/app/store/nodes';
import { PayoutsModule } from '@/app/store/payouts';
import { Nodes } from '@/nodes/service';
import { Payouts } from '@/payouts/service';
import { createLocalVue } from '@vue/test-utils';

const Vue = createLocalVue();

Vue.use(Vuex);

const nodesClient: NodesClient = new NodesClient();
export const nodesService: Nodes = new Nodes(nodesClient);
const nodesModule: NodesModule = new NodesModule(nodesService);
const payoutsClient: PayoutsClient = new PayoutsClient();
export const payoutsService: Payouts = new Payouts(payoutsClient);
const payoutsModule: PayoutsModule = new PayoutsModule(payoutsService);

const store = new Vuex.Store({ modules: { payouts: payoutsModule, nodes: nodesModule }});

export default store;

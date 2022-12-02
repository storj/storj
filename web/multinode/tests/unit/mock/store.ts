// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue } from '@vue/test-utils';

import { BandwidthClient } from '@/api/bandwidth';
import { NodesClient } from '@/api/nodes';
import { Operators as OperatorsClient } from '@/api/operators';
import { PayoutsClient } from '@/api/payouts';
import { StorageClient } from '@/api/storage';
import { BandwidthModule } from '@/app/store/bandwidth';
import { NodesModule } from '@/app/store/nodes';
import { OperatorsModule } from '@/app/store/operators';
import { PayoutsModule } from '@/app/store/payouts';
import { StorageModule } from '@/app/store/storage';
import { Bandwidth } from '@/bandwidth/service';
import { Nodes } from '@/nodes/service';
import { Operators } from '@/operators';
import { Payouts } from '@/payouts/service';
import { StorageService } from '@/storage/service';

const Vue = createLocalVue();

Vue.use(Vuex);

const nodesClient: NodesClient = new NodesClient();

export const nodesService: Nodes = new Nodes(nodesClient);

const nodesModule: NodesModule = new NodesModule(nodesService);

const payoutsClient: PayoutsClient = new PayoutsClient();

export const payoutsService: Payouts = new Payouts(payoutsClient);

const payoutsModule: PayoutsModule = new PayoutsModule(payoutsService);

const bandwidthClient = new BandwidthClient();

export const bandwidthService = new Bandwidth(bandwidthClient);

const bandwidthModule: BandwidthModule = new BandwidthModule(bandwidthService);

const operatorsClient: OperatorsClient = new OperatorsClient();

export const operatorsService: Operators = new Operators(operatorsClient);

const operatorsModule: OperatorsModule = new OperatorsModule(operatorsService);

const storageClient = new StorageClient();

export const storageService = new StorageService(storageClient);

const storageModule: StorageModule = new StorageModule(storageService);

const store = new Vuex.Store({ modules: {
    payouts: payoutsModule,
    nodes: nodesModule,
    operators: operatorsModule,
    bandwidth: bandwidthModule,
    storage: storageModule,
} });

export default store;

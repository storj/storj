// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';

import { BandwidthClient } from '@/api/bandwidth';
import { NodesClient } from '@/api/nodes';
import { Operators as OperatorsClient } from '@/api/operators';
import { PayoutsClient } from '@/api/payouts';
import { StorageClient } from '@/api/storage';
import { BandwidthModule } from '@/app/store/bandwidth';
import { NodesModule } from '@/app/store/nodes';
import { NotificationsModule } from '@/app/store/notifications';
import { OperatorsModule } from '@/app/store/operators';
import { PayoutsModule } from '@/app/store/payouts';
import { StorageModule } from '@/app/store/storage';
import { Bandwidth } from '@/bandwidth/service';
import { Nodes } from '@/nodes/service';
import { Operators } from '@/operators';
import { Payouts } from '@/payouts/service';
import { StorageService } from '@/storage/service';

Vue.use(Vuex);

// Services
const nodesClient = new NodesClient();
const nodesService = new Nodes(nodesClient);
const payoutsClient = new PayoutsClient();
const payoutsService = new Payouts(payoutsClient);
const bandwidthClient = new BandwidthClient();
const bandwidthService = new Bandwidth(bandwidthClient);
const operatorsClient = new OperatorsClient();
const operatorsService = new Operators(operatorsClient);
const storageClient = new StorageClient();
const storageService = new StorageService(storageClient);

// Modules
const nodesModule = new NodesModule(nodesService);
const payoutsModule = new PayoutsModule(payoutsService);
const bandwidthModule = new BandwidthModule(bandwidthService);
const operatorsModule = new OperatorsModule(operatorsService);
const storageModule = new StorageModule(storageService);
const notificationModule = new NotificationsModule();

export abstract class RootState {
    nodes: typeof nodesModule.state;
    payouts: typeof payoutsModule.state;
    bandwidth: typeof bandwidthModule.state;
    operators: typeof operatorsModule.state;
    storage: typeof storageModule.state;
    notification: typeof notificationModule.state;
}

// Store
export const store = new Vuex.Store<RootState>({
    strict: true,
    modules: {
        nodes: nodesModule,
        payouts: payoutsModule,
        bandwidth: bandwidthModule,
        operators: operatorsModule,
        storage: storageModule,
        notification: notificationModule,
    },
});

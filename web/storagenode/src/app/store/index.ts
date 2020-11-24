// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';

import { newNotificationsModule } from '@/app/store/modules/notifications';
import { newPayoutModule } from '@/app/store/modules/payout';
import { NotificationsHttpApi } from '@/storagenode/api/notifications';
import { PayoutHttpApi } from '@/storagenode/api/payout';
import { StorageNodeApi } from '@/storagenode/api/storagenode';
import { NotificationsService } from '@/storagenode/notifications/service';
import { PayoutService } from '@/storagenode/payouts/service';
import { StorageNodeService } from '@/storagenode/sno/service';

import { appStateModule } from './modules/appState';
import { newNodeModule } from './modules/node';

const notificationsApi = new NotificationsHttpApi();
const notificationsService = new NotificationsService(notificationsApi);
const payoutApi = new PayoutHttpApi();
const payoutService = new PayoutService(payoutApi);
const nodeApi = new StorageNodeApi();
const nodeService = new StorageNodeService(nodeApi);

Vue.use(Vuex);

export class StoreModule<S> {
    public state: S;
    public mutations: any;
    public actions: any;
    public getters?: any;
}

/**
 * storage node store (vuex)
 */
export const store = new Vuex.Store({
   modules: {
       node: newNodeModule(nodeService),
       appStateModule,
       notificationsModule: newNotificationsModule(notificationsService),
       payoutModule: newPayoutModule(payoutService),
   },
});

export default store;

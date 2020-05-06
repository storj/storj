// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';

import { makeNotificationsModule } from '@/app/store/modules/notifications';
import { makePayoutModule } from '@/app/store/modules/payout';
import { NotificationsHttpApi } from '@/storagenode/api/notifications';
import { PayoutHttpApi } from '@/storagenode/api/payout';
import { SNOApi } from '@/storagenode/api/storagenode';

import { appStateModule } from './modules/appState';
import { makeNodeModule } from './modules/node';

const notificationsApi = new NotificationsHttpApi();
const payoutApi = new PayoutHttpApi();
const nodeApi = new SNOApi();

Vue.use(Vuex);

/**
 * storage node store (vuex)
 */
export const store = new Vuex.Store({
   modules: {
       node: makeNodeModule(nodeApi),
       appStateModule,
       notificationsModule: makeNotificationsModule(notificationsApi),
       payoutModule: makePayoutModule(payoutApi),
   },
});

export default store;

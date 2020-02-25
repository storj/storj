// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';

import { makeNotificationsModule } from '@/app/store/modules/notifications';
import { NotificationsHttpApi } from '@/storagenode/api/notifications';

import { appStateModule } from './modules/appState';
import { node } from './modules/node';

const notificationsApi = new NotificationsHttpApi();

Vue.use(Vuex);

/**
 * storage node store (vuex)
 */
export const store = new Vuex.Store({
   modules: {
       node,
       appStateModule,
       notificationsModule: makeNotificationsModule(notificationsApi),
   },
});

export default store;

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';

import { appStateModule } from './modules/appState';
import { node } from './modules/node';

Vue.use(Vuex);

// storage node store (vuex)
const store = new Vuex.Store({
   modules: {
       node,
       appStateModule,
   }
});

export default store;

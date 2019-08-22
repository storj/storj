// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';
import { node } from './modules/node';
import { appStateModule } from './modules/appState';

Vue.use(Vuex);

// storage node store (vuex)
const store = new Vuex.Store({
   modules: {
       node,
       appStateModule,
   }
});

export default store;

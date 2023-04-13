// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';

import { FilesState, makeFilesModule } from '@/store/modules/files';

Vue.use(Vuex);

export interface ModulesState {
    files: FilesState;
}

// Satellite store (vuex)
export const store = new Vuex.Store<ModulesState>({
    modules: {
        files: makeFilesModule(),
    },
});

export default store;

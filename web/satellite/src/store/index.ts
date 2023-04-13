// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';

import { makeNotificationsModule, NotificationsState } from '@/store/modules/notifications';
import { FilesState, makeFilesModule } from '@/store/modules/files';

Vue.use(Vuex);

export interface ModulesState {
    notificationsModule: NotificationsState;
    files: FilesState;
}

// Satellite store (vuex)
export const store = new Vuex.Store<ModulesState>({
    modules: {
        notificationsModule: makeNotificationsModule(),
        files: makeFilesModule(),
    },
});

export default store;

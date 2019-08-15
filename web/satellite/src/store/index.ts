// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';

import { makeUsersModule } from '@/store/modules/users';
import { projectsModule } from '@/store/modules/projects';
import { projectMembersModule } from '@/store/modules/projectMembers';
import { notificationsModule } from '@/store/modules/notifications';
import { appStateModule } from '@/store/modules/appState';
import { makeApiKeysModule } from '@/store/modules/apiKeys';
import { bucketUsageModule, usageModule, creditUsageModule } from '@/store/modules/usage';
import { projectPaymentsMethodsModule } from '@/store/modules/paymentMethods';
import { UsersApiGql } from '@/api/users';

Vue.use(Vuex);

export class StoreModule<S> {
    public state: S;
    public mutations: any;
    public actions: any;
    public getters: any;
}

const usersApi = new UsersApiGql();

// Satellite store (vuex)
const store = new Vuex.Store({
    modules: {
        usersModule: makeUsersModule(usersApi),
        projectsModule,
        projectMembersModule,
        notificationsModule,
        appStateModule,
        apiKeysModule: makeApiKeysModule(),
        usageModule,
        bucketUsageModule,
        projectPaymentsMethodsModule,
        creditUsageModule
    }
});

export default store;

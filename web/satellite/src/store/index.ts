// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';

import { ApiKeysApiGql } from '@/api/apiKeys';
import { ProjectMembersApiGql } from '@/api/projectMembers';
import { UsersApiGql } from '@/api/users';
import { makeApiKeysModule } from '@/store/modules/apiKeys';
import { appStateModule } from '@/store/modules/appState';
import { notificationsModule } from '@/store/modules/notifications';
import { projectPaymentsMethodsModule } from '@/store/modules/paymentMethods';
import { makeProjectMembersModule } from '@/store/modules/projectMembers';
import { makeProjectsModule } from '@/store/modules/projects';
import { bucketUsageModule, usageModule } from '@/store/modules/usage';
import { makeUsersModule } from '@/store/modules/users';

Vue.use(Vuex);

export class StoreModule<S> {
    public state: S;
    public mutations: any;
    public actions: any;
    public getters: any;
}

const usersApi = new UsersApiGql();
const apiKeysApi = new ApiKeysApiGql();
const projectMembersApi = new ProjectMembersApiGql();

// Satellite store (vuex)
const store = new Vuex.Store({
    modules: {
        apiKeysModule: makeApiKeysModule(apiKeysApi),
        appStateModule,
        bucketUsageModule,
        projectMembersModule: makeProjectMembersModule(projectMembersApi),
        projectPaymentsMethodsModule,
        projectsModule: makeProjectsModule(),
        notificationsModule,
        usageModule,
        usersModule: makeUsersModule(usersApi),
    }
});

export default store;

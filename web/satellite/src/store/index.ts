// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';

import { CreditsApiGql } from '@/api/credits';
import { UsersApiGql } from '@/api/users';
import { ApiKeysApiGql } from '@/api/apiKeys';
import { BucketsApiGql } from '@/api/buckets';
import { ProjectMembersApiGql } from '@/api/projectMembers';
import { appStateModule } from '@/store/modules/appState';
import { makeApiKeysModule } from '@/store/modules/apiKeys';
import { makeBucketsModule } from '@/store/modules/buckets';
import { makeCreditsModule } from '@/store/modules/credits';
import { makeProjectsModule } from '@/store/modules/projects';
import { makeProjectMembersModule } from '@/store/modules/projectMembers';
import { makeUsersModule } from '@/store/modules/users';
import { notificationsModule } from '@/store/modules/notifications';
import { projectPaymentsMethodsModule } from '@/store/modules/paymentMethods';
import { usageModule } from '@/store/modules/usage';

Vue.use(Vuex);

export class StoreModule<S> {
    public state: S;
    public mutations: any;
    public actions: any;
    public getters?: any;
}

// TODO: remove it after we will use modules as classes and use some DI framework
const usersApi = new UsersApiGql();
const apiKeysApi = new ApiKeysApiGql();
const creditsApi = new CreditsApiGql();
const bucketsApi = new BucketsApiGql();
const projectMembersApi = new ProjectMembersApiGql();

// Satellite store (vuex)
const store = new Vuex.Store({
    modules: {
        usersModule: makeUsersModule(usersApi),
        projectsModule: makeProjectsModule(),
        projectMembersModule: makeProjectMembersModule(projectMembersApi),
        notificationsModule,
        appStateModule,
        apiKeysModule: makeApiKeysModule(apiKeysApi),
        usageModule,
        bucketUsageModule: makeBucketsModule(bucketsApi),
        projectPaymentsMethodsModule,
        creditsModule: makeCreditsModule(creditsApi),
    }
});

export default store;

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';

import { makeNotificationsModule } from '@/store/modules/notifications';
import { ApiKeysApiGql } from '@/api/apiKeys';
import { BucketsApiGql } from '@/api/buckets';
import { CreditsApiGql } from '@/api/credits';
import { ProjectMembersApiGql } from '@/api/projectMembers';
import { UsersApiGql } from '@/api/users';
import { appStateModule } from '@/store/modules/appState';
import { makeApiKeysModule } from '@/store/modules/apiKeys';
import { makeCreditsModule } from '@/store/modules/credits';
import { makeBucketsModule } from '@/store/modules/buckets';
import { projectPaymentsMethodsModule } from '@/store/modules/paymentMethods';
import { makeProjectMembersModule } from '@/store/modules/projectMembers';
import { makeProjectsModule } from '@/store/modules/projects';
import { usageModule } from '@/store/modules/usage';
import { makeUsersModule } from '@/store/modules/users';
import { ProjectsApiGql } from '@/api/projects';

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
const projectsApi = new ProjectsApiGql();

// Satellite store (vuex)
const store = new Vuex.Store({
    modules: {
        notificationsModule: makeNotificationsModule(),
        apiKeysModule: makeApiKeysModule(apiKeysApi),
        appStateModule,
        creditsModule: makeCreditsModule(creditsApi),
        projectMembersModule: makeProjectMembersModule(projectMembersApi),
        projectPaymentsMethodsModule,
        usersModule: makeUsersModule(usersApi),
        projectsModule: makeProjectsModule(projectsApi),
        usageModule,
        bucketUsageModule: makeBucketsModule(bucketsApi),
    }
});

export default store;

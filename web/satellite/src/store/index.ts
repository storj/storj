// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';

import { AccessGrantsApiGql } from '@/api/accessGrants';
import { AuthHttpApi } from '@/api/auth';
import { BucketsApiGql } from '@/api/buckets';
import { PaymentsHttpApi } from '@/api/payments';
import { ProjectMembersApiGql } from '@/api/projectMembers';
import { ProjectsApiGql } from '@/api/projects';
import { notProjectRelatedRoutes, router } from '@/router';
import { AccessGrantsState, makeAccessGrantsModule } from '@/store/modules/accessGrants';
import { appStateModule } from '@/store/modules/appState';
import { makeBucketsModule } from '@/store/modules/buckets';
import { makeNotificationsModule, NotificationsState } from '@/store/modules/notifications';
import { makePaymentsModule, PaymentsState } from '@/store/modules/payments';
import { makeProjectMembersModule, ProjectMembersState } from '@/store/modules/projectMembers';
import { makeProjectsModule, ProjectsState, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { makeUsersModule } from '@/store/modules/users';
import { User } from '@/types/users';

Vue.use(Vuex);

export class StoreModule<S> {
    public state: S;
    public mutations: any;
    public actions: any;
    public getters?: any;
}

// TODO: remove it after we will use modules as classes and use some DI framework
const authApi = new AuthHttpApi();
const accessGrantsApi = new AccessGrantsApiGql();
const bucketsApi = new BucketsApiGql();
const projectMembersApi = new ProjectMembersApiGql();
const projectsApi = new ProjectsApiGql();
const paymentsApi = new PaymentsHttpApi();

class ModulesState {
    public notificationsModule: NotificationsState;
    public accessGrantsModule: AccessGrantsState;
    public appStateModule;
    public projectMembersModule: ProjectMembersState;
    public paymentsModule: PaymentsState;
    public usersModule: User;
    public projectsModule: ProjectsState;
}

// Satellite store (vuex)
export const store = new Vuex.Store<ModulesState>({
    modules: {
        notificationsModule: makeNotificationsModule(),
        accessGrantsModule: makeAccessGrantsModule(accessGrantsApi),
        appStateModule,
        projectMembersModule: makeProjectMembersModule(projectMembersApi),
        paymentsModule: makePaymentsModule(paymentsApi),
        usersModule: makeUsersModule(authApi),
        projectsModule: makeProjectsModule(projectsApi),
        bucketUsageModule: makeBucketsModule(bucketsApi),
    },
});

store.subscribe((mutation, state) => {
    const currentRouteName = router.currentRoute.name;
    const satelliteName = state.appStateModule.satelliteName;

    switch (mutation.type) {
        case PROJECTS_MUTATIONS.REMOVE:
            document.title = `${router.currentRoute.name} | ${satelliteName}`;

            break;
        case PROJECTS_MUTATIONS.SELECT_PROJECT:
            if (currentRouteName && !notProjectRelatedRoutes.includes(currentRouteName)) {
                document.title = `${state.projectsModule.selectedProject.name} | ${currentRouteName} | ${satelliteName}`;
            }
    }
});

export default store;

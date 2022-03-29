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
import { makeObjectsModule, ObjectsState } from '@/store/modules/objects';
import { makePaymentsModule, PaymentsState } from '@/store/modules/payments';
import { makeProjectMembersModule, ProjectMembersState } from '@/store/modules/projectMembers';
import { makeProjectsModule, PROJECTS_MUTATIONS, ProjectsState } from '@/store/modules/projects';
import { makeUsersModule } from '@/store/modules/users';
import { User } from '@/types/users';

import { FilesState, makeFilesModule } from '@/store/modules/files';

Vue.use(Vuex);

type Mutation<State> =
    (state: State, ...args: any[]) => any; // eslint-disable-line @typescript-eslint/no-explicit-any

type Action<Context> =
    (context: Context, ...args: any[]) => (Promise<any> | void | any); // eslint-disable-line @typescript-eslint/no-explicit-any

type Getter<State, Context> =
    Context extends {rootGetters: any} ? ( // eslint-disable-line @typescript-eslint/no-explicit-any
        ((state: State) => any) | // eslint-disable-line @typescript-eslint/no-explicit-any
        ((state: State, rootGetters: Context["rootGetters"]) => any) // eslint-disable-line @typescript-eslint/no-explicit-any
    ) : ((state: State) => any); // eslint-disable-line @typescript-eslint/no-explicit-any

export interface StoreModule<State, Context> { // eslint-disable-line @typescript-eslint/no-unused-vars
    state: State;
    mutations: Record<string, Mutation<State>>
    actions: Record<string, Action<Context>>
    getters?: Record<string, Getter<State, Context>>
}

// TODO: remove it after we will use modules as classes and use some DI framework
const authApi = new AuthHttpApi();
const accessGrantsApi = new AccessGrantsApiGql();
const bucketsApi = new BucketsApiGql();
const projectMembersApi = new ProjectMembersApiGql();
const projectsApi = new ProjectsApiGql();
const paymentsApi = new PaymentsHttpApi();

export interface ModulesState {
    notificationsModule: NotificationsState;
    accessGrantsModule: AccessGrantsState;
    appStateModule: typeof appStateModule.state;
    projectMembersModule: ProjectMembersState;
    paymentsModule: PaymentsState;
    usersModule: User;
    projectsModule: ProjectsState;
    objectsModule: ObjectsState;
    files: FilesState;
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
        objectsModule: makeObjectsModule(),
        files: makeFilesModule(),
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

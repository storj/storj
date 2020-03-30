// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';

import { ApiKeysApiGql } from '@/api/apiKeys';
import { AuthHttpApi } from '@/api/auth';
import { BucketsApiGql } from '@/api/buckets';
import { CreditsApiGql } from '@/api/credits';
import { PaymentsHttpApi } from '@/api/payments';
import { ProjectMembersApiGql } from '@/api/projectMembers';
import { ProjectsApiGql } from '@/api/projects';
import { ReferralHttpApi } from '@/api/referral';
import { notProjectRelatedRoutes, router } from '@/router';
import { ApiKeysState, makeApiKeysModule } from '@/store/modules/apiKeys';
import { appStateModule } from '@/store/modules/appState';
import { makeBucketsModule } from '@/store/modules/buckets';
import { makeCreditsModule } from '@/store/modules/credits';
import { makeNotificationsModule, NotificationsState } from '@/store/modules/notifications';
import { makePaymentsModule, PaymentsState } from '@/store/modules/payments';
import { makeProjectMembersModule, ProjectMembersState } from '@/store/modules/projectMembers';
import { makeProjectsModule, PROJECTS_MUTATIONS, ProjectsState } from '@/store/modules/projects';
import { makeReferralModule, ReferralState } from '@/store/modules/referral';
import { makeUsersModule } from '@/store/modules/users';
import { CreditUsage } from '@/types/credits';
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
const apiKeysApi = new ApiKeysApiGql();
const creditsApi = new CreditsApiGql();
const bucketsApi = new BucketsApiGql();
const projectMembersApi = new ProjectMembersApiGql();
const projectsApi = new ProjectsApiGql();
const paymentsApi = new PaymentsHttpApi();
const referralApi = new ReferralHttpApi();

class ModulesState {
    public notificationsModule: NotificationsState;
    public apiKeysModule: ApiKeysState;
    public appStateModule;
    public creditsModule: CreditUsage;
    public projectMembersModule: ProjectMembersState;
    public paymentsModule: PaymentsState;
    public usersModule: User;
    public projectsModule: ProjectsState;
    public referralModule: ReferralState;
}

// Satellite store (vuex)
export const store = new Vuex.Store<ModulesState>({
    modules: {
        notificationsModule: makeNotificationsModule(),
        apiKeysModule: makeApiKeysModule(apiKeysApi),
        appStateModule,
        creditsModule: makeCreditsModule(creditsApi),
        projectMembersModule: makeProjectMembersModule(projectMembersApi),
        paymentsModule: makePaymentsModule(paymentsApi),
        usersModule: makeUsersModule(authApi),
        projectsModule: makeProjectsModule(projectsApi),
        bucketUsageModule: makeBucketsModule(bucketsApi),
        referralModule: makeReferralModule(referralApi),
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

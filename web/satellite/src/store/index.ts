// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';
import { RouteRecord } from 'vue-router';

import { AccessGrantsApiGql } from '@/api/accessGrants';
import { BucketsApiGql } from '@/api/buckets';
import { ProjectMembersApiGql } from '@/api/projectMembers';
import { ProjectsApiGql } from '@/api/projects';
import { notProjectRelatedRoutes, RouteConfig, router } from '@/router';
import { AccessGrantsState, makeAccessGrantsModule } from '@/store/modules/accessGrants';
import { makeAppStateModule, AppState } from '@/store/modules/appState';
import { BucketsState, makeBucketsModule } from '@/store/modules/buckets';
import { makeNotificationsModule, NotificationsState } from '@/store/modules/notifications';
import { makeObjectsModule, OBJECTS_ACTIONS, ObjectsState } from '@/store/modules/objects';
import { makeProjectsModule, PROJECTS_MUTATIONS, ProjectsState } from '@/store/modules/projects';
import { FilesState, makeFilesModule } from '@/store/modules/files';
import { NavigationLink } from '@/types/navigation';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { FrontendConfigHttpApi } from '@/api/config';

Vue.use(Vuex);

const accessGrantsApi = new AccessGrantsApiGql();
const bucketsApi = new BucketsApiGql();
const projectsApi = new ProjectsApiGql();
const configApi = new FrontendConfigHttpApi();

// We need to use a WebWorker factory because jest testing does not support
// WebWorkers yet. This is a way to avoid a direct dependency to `new Worker`.
const webWorkerFactory = {
    create(): Worker {
        return new Worker(new URL('@/utils/accessGrant.worker.js', import.meta.url), { type: 'module' });
    },
};

export interface ModulesState {
    notificationsModule: NotificationsState;
    accessGrantsModule: AccessGrantsState;
    appStateModule: AppState;
    projectsModule: ProjectsState;
    objectsModule: ObjectsState;
    bucketUsageModule: BucketsState;
    files: FilesState;
}

// Satellite store (vuex)
export const store = new Vuex.Store<ModulesState>({
    modules: {
        notificationsModule: makeNotificationsModule(),
        accessGrantsModule: makeAccessGrantsModule(accessGrantsApi, webWorkerFactory),
        appStateModule: makeAppStateModule(configApi),
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

/*
  These router methods have been moved here to avoid circular imports between
  store and the router. Many of the tests require router, however, this implementation
  relies on store state for the routing behavior.
*/
router.beforeEach(async (to, from, next) => {
    if (to.name === RouteConfig.ProjectDashboard.name && from.name === RouteConfig.Login.name) {
        store.commit(APP_STATE_MUTATIONS.TOGGLE_HAS_JUST_LOGGED_IN, true);
    }

    if (to.name === RouteConfig.AllProjectsDashboard.name && from.name === RouteConfig.Login.name) {
        store.commit(APP_STATE_MUTATIONS.TOGGLE_HAS_JUST_LOGGED_IN, true);
    }

    // On very first login we try to redirect user to project dashboard
    // but since there is no project we then redirect user to onboarding flow.
    // That's why we toggle this flag here back to false not show create project passphrase modal again
    // if user clicks 'Continue in web'.
    if (to.name === RouteConfig.ProjectDashboard.name && from.name === RouteConfig.OverviewStep.name) {
        store.commit(APP_STATE_MUTATIONS.TOGGLE_HAS_JUST_LOGGED_IN, false);
    }
    if (to.name === RouteConfig.ProjectDashboard.name && from.name === RouteConfig.AllProjectsDashboard.name) {
        store.commit(APP_STATE_MUTATIONS.TOGGLE_HAS_JUST_LOGGED_IN, false);
    }

    if (!to.path.includes(RouteConfig.UploadFile.path) && (store.state.appStateModule.viewsState.activeModal !== MODALS.uploadCancelPopup)) {
        const areUploadsInProgress: boolean = await store.dispatch(OBJECTS_ACTIONS.CHECK_ONGOING_UPLOADS, to.path);
        if (areUploadsInProgress) return;
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.Account)) {
        next(RouteConfig.Account.with(RouteConfig.Billing).path);

        return;
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep))) {
        next(RouteConfig.OnboardingTour.path);

        return;
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.OnboardingTour)) {
        next(RouteConfig.OnboardingTour.with(RouteConfig.FirstOnboardingStep).path);

        return;
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.Buckets)) {
        next(RouteConfig.Buckets.with(RouteConfig.BucketsManagement).path);

        return;
    }

    if (to.name === 'default') {
        next(RouteConfig.ProjectDashboard.path);

        return;
    }

    next();
});

router.afterEach(({ name }, _from) => {
    if (!name) {
        return;
    }

    if (notProjectRelatedRoutes.includes(name)) {
        document.title = `${router.currentRoute.name} | ${store.state.appStateModule.satelliteName}`;

        return;
    }

    const selectedProjectName = store.state.projectsModule.selectedProject.name ?
        `${store.state.projectsModule.selectedProject.name} | ` : '';

    document.title = `${selectedProjectName + router.currentRoute.name} | ${store.state.appStateModule.satelliteName}`;
});

/**
 * if our route is a tab and has no sub tab route - we will navigate to default subtab.
 * F.E. /account/ -> /account/billing/;
 * @param routes - array of RouteRecord from vue-router
 * @param tabRoute - tabNavigator route
 */
function navigateToDefaultSubTab(routes: RouteRecord[], tabRoute: NavigationLink): boolean {
    return (routes.length === 2 && (routes[1].name as string) === tabRoute.name) ||
        (routes.length === 3 && (routes[2].name as string) === tabRoute.name);
}

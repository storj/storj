// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';
import { RouteRecord } from 'vue-router';

import { BucketsApiGql } from '@/api/buckets';
import { ProjectsApiGql } from '@/api/projects';
import { notProjectRelatedRoutes, RouteConfig, router } from '@/router';
import { BucketsState, makeBucketsModule } from '@/store/modules/buckets';
import { makeNotificationsModule, NotificationsState } from '@/store/modules/notifications';
import { makeObjectsModule, ObjectsState } from '@/store/modules/objects';
import { makeProjectsModule, PROJECTS_MUTATIONS, ProjectsState } from '@/store/modules/projects';
import { FilesState, makeFilesModule } from '@/store/modules/files';
import { NavigationLink } from '@/types/navigation';
import { useAppStore } from '@/store/modules/appStore';

Vue.use(Vuex);

const bucketsApi = new BucketsApiGql();
const projectsApi = new ProjectsApiGql();

export interface ModulesState {
    notificationsModule: NotificationsState;
    projectsModule: ProjectsState;
    objectsModule: ObjectsState;
    bucketUsageModule: BucketsState;
    files: FilesState;
}

// Satellite store (vuex)
export const store = new Vuex.Store<ModulesState>({
    modules: {
        notificationsModule: makeNotificationsModule(),
        projectsModule: makeProjectsModule(projectsApi),
        bucketUsageModule: makeBucketsModule(bucketsApi),
        objectsModule: makeObjectsModule(),
        files: makeFilesModule(),
    },
});

store.subscribe((mutation) => {
    switch (mutation.type) {
    case PROJECTS_MUTATIONS.REMOVE:
    case PROJECTS_MUTATIONS.SELECT_PROJECT:
        updateTitle();
    }
});

export default store;

/*
  These router methods have been moved here to avoid circular imports between
  store and the router. Many of the tests require router, however, this implementation
  relies on store state for the routing behavior.
*/
router.beforeEach(async (to, from, next) => {
    const appStore = useAppStore();

    if (!to.matched.length) {
        appStore.setErrorPage(404);
        return;
    } else if (appStore.state.viewsState.error.visible) {
        appStore.removeErrorPage();
    }

    if (to.name === RouteConfig.ProjectDashboard.name && from.name === RouteConfig.Login.name) {
        appStore.toggleHasJustLoggedIn(true);
    }

    if (to.name === RouteConfig.AllProjectsDashboard.name && from.name === RouteConfig.Login.name) {
        appStore.toggleHasJustLoggedIn(true);
    }

    // On very first login we try to redirect user to project dashboard
    // but since there is no project we then redirect user to onboarding flow.
    // That's why we toggle this flag here back to false not show create project passphrase modal again
    // if user clicks 'Continue in web'.
    if (to.name === RouteConfig.ProjectDashboard.name && from.name === RouteConfig.OverviewStep.name) {
        appStore.toggleHasJustLoggedIn(false);
    }
    if (to.name === RouteConfig.ProjectDashboard.name && from.name === RouteConfig.AllProjectsDashboard.name) {
        appStore.toggleHasJustLoggedIn(false);
    }

    // TODO: I disabled this navigation guard because we try to get active pinia before it is initialised.
    // In any case this feature is redundant since we have project level passphrase.

    // if (!to.path.includes(RouteConfig.UploadFile.path)) {
    //     const appStore = useAppStore();
    //     if (appStore.state.viewsState.activeModal !== MODALS.uploadCancelPopup) {
    //         const areUploadsInProgress: boolean = await store.dispatch(OBJECTS_ACTIONS.CHECK_ONGOING_UPLOADS, to.path);
    //         if (areUploadsInProgress) return;
    //     }
    // }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.Account)) {
        next(RouteConfig.Account.with(RouteConfig.Billing).path);

        return;
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep))) {
        next(RouteConfig.OnboardingTour.path);

        return;
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.OnboardingTour)) {
        const firstOnboardingStep = appStore.state.config.pricingPackagesEnabled
            ? RouteConfig.PricingPlanStep
            : RouteConfig.OverviewStep;
        next(RouteConfig.OnboardingTour.with(firstOnboardingStep).path);

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

router.afterEach(() => {
    updateTitle();
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

/**
 * Updates the title of the webpage.
 */
function updateTitle(): void {
    const appStore = useAppStore();
    const routeName = router.currentRoute.name;
    const parts = [routeName, appStore.state.config.satelliteName];

    if (routeName && !notProjectRelatedRoutes.includes(routeName)) {
        parts.unshift(store.state.projectsModule.selectedProject.name);
    }

    document.title = parts.filter(s => !!s).join(' | ');
}

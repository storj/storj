// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <branded-loader v-if="isLoading" />
    <ErrorPage v-else-if="isErrorPageShown" />
    <router-view v-else />
    <Notifications />
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref } from 'vue';
import { useTheme } from 'vuetify';
import { useRoute, useRouter } from 'vue-router';

import { useConfigStore } from '@/store/modules/configStore';
import { useAppStore } from '@poc/store/appStore';
import { APIError } from '@/utils/error';
import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { useABTestingStore } from '@/store/modules/abTestingStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useNotify } from '@/utils/hooks';
import { useBillingStore } from '@/store/modules/billingStore';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { RouteConfig } from '@/types/router';
import { ROUTES } from '@poc/router';

import Notifications from '@poc/layouts/default/Notifications.vue';
import ErrorPage from '@poc/components/ErrorPage.vue';
import BrandedLoader from '@poc/components/utils/BrandedLoader.vue';

const appStore = useAppStore();
const abTestingStore = useABTestingStore();
const billingStore = useBillingStore();
const configStore = useConfigStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const analyticsStore = useAnalyticsStore();

const notify = useNotify();
const router = useRouter();
const theme = useTheme();
const route = useRoute();

const isLoading = ref<boolean>(true);

/**
 * Indicates whether an error page should be shown in place of the router view.
 */
const isErrorPageShown = computed<boolean>((): boolean => {
    return appStore.state.error.visible;
});

/**
 * Indicates if billing features are enabled.
 */
const billingEnabled = computed<boolean>(() => configStore.state.config.billingFeaturesEnabled);

/**
 * Sets up the app by fetching all necessary data.
 */
async function setup() {
    isLoading.value = true;
    try {
        await usersStore.getUser();
        const promises: Promise<void | object | string>[] = [
            usersStore.getSettings(),
            projectsStore.getProjects(),
            projectsStore.getUserInvitations(),
            abTestingStore.fetchValues(),
        ];
        if (billingEnabled.value) {
            promises.push(billingStore.setupAccount());
        }
        await Promise.all(promises);

        const invites = projectsStore.state.invitations;
        const projects = projectsStore.state.projects;

        if (appStore.state.hasJustLoggedIn && !invites.length && projects.length <= 1) {
            if (!projects.length) {
                await projectsStore.createDefaultProject(usersStore.state.user.id);
            } else {
                projectsStore.selectProject(projects[0].id);
                await router.push({
                    name: ROUTES.Dashboard.name,
                    params: { id: projectsStore.state.selectedProject.urlId },
                });
                analyticsStore.pageVisit(ROUTES.DashboardAnalyticsLink);
                analyticsStore.eventTriggered(AnalyticsEvent.NAVIGATE_PROJECTS);
            }
        }
    } catch (error) {
        if (!(error instanceof ErrorUnauthorized)) {
            notify.notifyError(error, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
            appStore.setErrorPage((error as APIError).status ?? 500, true);
        } else {
            await new Promise(resolve => setTimeout(resolve, 1000));
            if (!RouteConfig.AuthRoutes.includes(route.path)) await router.push(ROUTES.Login.path);
        }
    }
    isLoading.value = false;
}

/**
 * Lifecycle hook before initial render.
 * Sets up variables from meta tags from config such satellite name, etc.
 */
onBeforeMount(async (): Promise<void> => {
    const savedTheme = localStorage.getItem('theme') || 'light';
    if ((savedTheme === 'dark' && !theme.global.current.value.dark) || (savedTheme === 'light' && theme.global.current.value.dark)) {
        theme.global.name.value = savedTheme;
    }

    try {
        await configStore.getConfig();
    } catch (error) {
        isLoading.value = false;
        notify.notifyError(error, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
        appStore.setErrorPage((error as APIError).status ?? 500, true);
        return;
    }

    await setup();

    isLoading.value = false;
});

usersStore.$onAction(({ name, after }) => {
    if (name === 'login') {
        after((_) => setup());
    }
});
</script>

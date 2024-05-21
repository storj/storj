// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <branded-loader v-if="isLoading" />
    <ErrorPage v-else-if="isErrorPageShown" />
    <template v-else>
        <router-view />
        <trial-expiration-dialog
            v-if="!user.paidTier"
            v-model="appStore.state.isExpirationDialogShown"
            :expired="user.freezeStatus.trialExpiredFrozen"
        />
    </template>
    <Notifications />
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref, watch } from 'vue';
import { useTheme } from 'vuetify';
import { useRoute, useRouter } from 'vue-router';

import { useConfigStore } from '@/store/modules/configStore';
import { useAppStore } from '@/store/modules/appStore';
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
import { ROUTES } from '@/router';
import { User } from '@/types/users';
import { useBucketsStore } from '@/store/modules/bucketsStore';

import Notifications from '@/layouts/default/Notifications.vue';
import ErrorPage from '@/components/ErrorPage.vue';
import BrandedLoader from '@/components/utils/BrandedLoader.vue';
import TrialExpirationDialog from '@/components/dialogs/TrialExpirationDialog.vue';

const appStore = useAppStore();
const abTestingStore = useABTestingStore();
const billingStore = useBillingStore();
const bucketsStore = useBucketsStore();
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
 * Returns user entity from store.
 */
const user = computed<User>(() => usersStore.state.user);

/**
 * Sets up the app by fetching all necessary data.
 */
async function setup() {
    isLoading.value = true;
    const source = new URLSearchParams(window.location.search).get('source');
    try {
        await usersStore.getUser();
        const promises: Promise<void | object | string>[] = [
            usersStore.getSettings(),
            projectsStore.getProjects(),
            projectsStore.getUserInvitations(),
            abTestingStore.fetchValues(),
        ];
        if (configStore.state.config.billingFeaturesEnabled) {
            promises.push(billingStore.setupAccount());
        }
        await Promise.all(promises);

        const invites = projectsStore.state.invitations;
        const projects = projectsStore.state.projects;

        if (source) {
            analyticsStore.eventTriggered(AnalyticsEvent.ARRIVED_FROM_SOURCE, { source: source });
        }

        if (appStore.state.hasJustLoggedIn && !invites.length && projects.length <= 1) {
            if (!projects.length) {
                await projectsStore.createDefaultProject(usersStore.state.user.id);
            }
            projectsStore.selectProject(projects[0].id);
            const project = projectsStore.state.selectedProject;
            await router.push({
                name: ROUTES.Dashboard.name,
                params: { id: project.urlId },
            });
            analyticsStore.eventTriggered(AnalyticsEvent.NAVIGATE_PROJECTS);

            if (usersStore.getShouldPromptPassphrase({
                isProjectOwner: project.ownerId === usersStore.state.user.id,
                onboardingStepperEnabled: configStore.state.config.onboardingStepperEnabled,
            }) && !user.value.freezeStatus.trialExpiredFrozen) {
                appStore.toggleProjectPassphraseDialog(true);
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

    if (configStore.state.config.analyticsEnabled) {
        analyticsStore.pageVisit(route.matched[route.matched.length - 1].path, configStore.state.config.satelliteName);
    }
});

usersStore.$onAction(({ name, after }) => {
    if (name === 'login') {
        after((_) => {
            setup().then(() => {
                if (user.value.paidTier || route.name !== ROUTES.Dashboard.name || projectsStore.state.selectedProject.ownerId !== user.value.id) return;

                const expirationInfo = user.value.getExpirationInfo(configStore.state.config.daysBeforeTrialEndNotification);
                if (user.value.freezeStatus.trialExpiredFrozen || expirationInfo.isCloseToExpiredTrial) {
                    appStore.toggleExpirationDialog(true);
                }
            });
        });
    }
});

bucketsStore.$onAction(({ name, after, args }) => {
    if (name === 'handleDeleteBucketRequest') {
        after(async (_) => {
            const bucketName = args[0];
            const request = args[1];
            try {
                await request;
                analyticsStore.eventTriggered(AnalyticsEvent.BUCKET_DELETED);
                notify.success(`Successfully deleted ${bucketName}.`, 'Bucket Deleted');
            } catch (error) {
                let message = `Failed to delete ${bucketName}.`;
                if (error && error.message) {
                    message += ` ${error.message}`;
                }
                notify.error(message, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
            }

            try {
                await bucketsStore.getBuckets(1, projectsStore.state.selectedProject.id);
            } catch (error) {
                notify.notifyError(error, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
            }
        });
    }
});

/**
 * conditionally prompt for project passphrase if project changes
 */
watch(() => projectsStore.state.selectedProject, (project, oldProject) => {
    if (project.id === oldProject.id) {
        return;
    }
    if (usersStore.getShouldPromptPassphrase({
        isProjectOwner: project.ownerId === usersStore.state.user.id,
        onboardingStepperEnabled: configStore.state.config.onboardingStepperEnabled,
    }) && !user.value.freezeStatus.trialExpiredFrozen && route.name !== ROUTES.Bucket.name) {
        appStore.toggleProjectPassphraseDialog(true);
    }
});
</script>

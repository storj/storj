// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app>
        <div v-if="isLoading" class="d-flex align-center justify-center w-100 h-100">
            <v-progress-circular color="primary" indeterminate size="64" />
        </div>
        <session-wrapper v-else>
            <default-bar show-nav-drawer-button />
            <ProjectNav />
            <default-view />

            <UpgradeAccountDialog v-model="appStore.state.isUpgradeFlowDialogShown" />
            <browser-snackbar-component />
        </session-wrapper>
    </v-app>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { VApp, VProgressCircular } from 'vuetify/components';

import DefaultBar from './AppBar.vue';
import ProjectNav from './ProjectNav.vue';
import DefaultView from './View.vue';

import { RouteConfig } from '@/types/router';
import { Project } from '@/types/projects';
import { useBillingStore } from '@/store/modules/billingStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useABTestingStore } from '@/store/modules/abTestingStore';
import { MINIMUM_URL_ID_LENGTH, useProjectsStore } from '@/store/modules/projectsStore';
import { useAppStore } from '@poc/store/appStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useConfigStore } from '@/store/modules/configStore';

import SessionWrapper from '@poc/components/utils/SessionWrapper.vue';
import UpgradeAccountDialog from '@poc/components/dialogs/upgradeAccountFlow/UpgradeAccountDialog.vue';
import BrowserSnackbarComponent from '@poc/components/BrowserSnackbarComponent.vue';

const router = useRouter();
const route = useRoute();
const notify = useNotify();

const analyticsStore = useAnalyticsStore();
const billingStore = useBillingStore();
const usersStore = useUsersStore();
const abTestingStore = useABTestingStore();
const projectsStore = useProjectsStore();
const appStore = useAppStore();
const agStore = useAccessGrantsStore();
const configStore = useConfigStore();

const isLoading = ref<boolean>(true);

/**
 * Indicates if billing features are enabled.
 */
const billingEnabled = computed<boolean>(() => configStore.state.config.billingFeaturesEnabled);

/**
 * Selects the project with the given URL ID, redirecting to the
 * all projects dashboard if no such project exists.
 */
async function selectProject(urlId: string): Promise<void> {
    const goToDashboard = () => {
        const path = '/projects';
        router.push(path);
        analyticsStore.pageVisit(path);
    };

    if (urlId.length < MINIMUM_URL_ID_LENGTH) {
        goToDashboard();
        return;
    }

    let projects: Project[];
    try {
        projects = await projectsStore.getProjects();
    } catch (_) {
        goToDashboard();
        return;
    }

    const project = projects.find(p => {
        let prefixEnd = 0;
        while (
            p.urlId[prefixEnd] === urlId[prefixEnd]
            && prefixEnd < p.urlId.length
            && prefixEnd < urlId.length
        ) prefixEnd++;
        return prefixEnd === p.urlId.length || prefixEnd === urlId.length;
    });
    if (!project) {
        goToDashboard();
        return;
    }
    projectsStore.selectProject(project.id);
}

watch(() => route.params.id, async newId => {
    if (newId === undefined) return;
    isLoading.value = true;
    await selectProject(newId as string);
    isLoading.value = false;
});

/**
 * Lifecycle hook after initial render.
 * Pre-fetches user`s and project information.
 */
onBeforeMount(async () => {
    isLoading.value = true;

    try {
        await Promise.all([
            usersStore.getUser(),
            abTestingStore.fetchValues(),
            usersStore.getSettings(),
        ]);
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
        setTimeout(async () => await router.push(RouteConfig.Login.path), 1000);

        return;
    }

    if (billingEnabled.value) {
        try {
            await billingStore.setupAccount();
        } catch (error) {
            error.message = `Unable to setup account. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
        }
    }

    await selectProject(route.params.id as string);

    if (!agStore.state.accessGrantsWebWorker) await agStore.startWorker();

    isLoading.value = false;
});
</script>

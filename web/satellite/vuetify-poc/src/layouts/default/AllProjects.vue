// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app>
        <div v-if="isLoading" class="d-flex align-center justify-center w-100 h-100">
            <v-progress-circular color="primary" indeterminate size="64" />
        </div>
        <session-wrapper v-else>
            <default-bar />
            <default-view />

            <UpgradeAccountDialog v-model="appStore.state.isUpgradeFlowDialogShown" />
            <browser-snackbar-component />
        </session-wrapper>
    </v-app>
</template>

<script setup lang="ts">
import { VApp, VProgressCircular } from 'vuetify/components';
import { onBeforeMount, onBeforeUnmount, ref } from 'vue';
import { useRouter } from 'vue-router';

import DefaultBar from './AppBar.vue';
import DefaultView from './View.vue';

import { useAppStore } from '@poc/store/appStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import SessionWrapper from '@poc/components/utils/SessionWrapper.vue';
import UpgradeAccountDialog from '@poc/components/dialogs/upgradeAccountFlow/UpgradeAccountDialog.vue';
import BrowserSnackbarComponent from '@poc/components/BrowserSnackbarComponent.vue';

const appStore = useAppStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const analyticsStore = useAnalyticsStore();

const notify = useNotify();
const router = useRouter();

const isLoading = ref<boolean>(true);

/**
 * Lifecycle hook after initial render.
 * Pre-fetches user`s and project information.
 */
onBeforeMount(async () => {
    try {
        await usersStore.getSettings();
        await usersStore.getUser();
        await projectsStore.getProjects();
        const invites = await projectsStore.getUserInvitations();

        const projects = projectsStore.state.projects;

        if (appStore.state.hasJustLoggedIn && !invites.length && projects.length <= 1) {
            if (!projects.length) {
                await projectsStore.createDefaultProject(usersStore.state.user.id);
            } else {
                projectsStore.selectProject(projects[0].id);
                await router.push(`/projects/${projectsStore.state.selectedProject.urlId}/dashboard`);
                analyticsStore.pageVisit('/projects/dashboard');
                analyticsStore.eventTriggered(AnalyticsEvent.NAVIGATE_PROJECTS);
            }
        }
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.ALL_PROJECT_DASHBOARD);
    }

    isLoading.value = false;
});

onBeforeUnmount(() => {
    appStore.toggleHasJustLoggedIn(false);
});
</script>

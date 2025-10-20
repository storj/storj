// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app>
        <branded-loader v-if="isLoading" />
        <session-wrapper v-else>
            <default-bar show-nav-drawer-button />
            <ProjectNav />
            <default-view />

            <UpgradeAccountDialog v-model="appStore.state.isUpgradeFlowDialogShown" :is-member-upgrade="isMemberAccount" />
            <browser-snackbar-component />
        </session-wrapper>
    </v-app>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { VApp } from 'vuetify/components';

import DefaultBar from './AppBar.vue';
import ProjectNav from './ProjectNav.vue';
import DefaultView from './View.vue';

import { Project } from '@/types/projects';
import { MINIMUM_URL_ID_LENGTH, useProjectsStore } from '@/store/modules/projectsStore';
import { useAppStore } from '@/store/modules/appStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { ROUTES } from '@/router';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useAccessGrantWorker } from '@/composables/useAccessGrantWorker';
import { useUsersStore } from '@/store/modules/usersStore';

import SessionWrapper from '@/components/utils/SessionWrapper.vue';
import UpgradeAccountDialog from '@/components/dialogs/upgradeAccountFlow/UpgradeAccountDialog.vue';
import BrowserSnackbarComponent from '@/components/BrowserSnackbarComponent.vue';
import BrandedLoader from '@/components/utils/BrandedLoader.vue';

const router = useRouter();
const route = useRoute();
const notify = useNotify();
const { start } = useAccessGrantWorker();

const bucketsStore = useBucketsStore();
const projectsStore = useProjectsStore();
const appStore = useAppStore();
const agStore = useAccessGrantsStore();
const usersStore = useUsersStore();

const isLoading = ref<boolean>(true);

const isMemberAccount = computed<boolean>(() => usersStore.state.user.isMember);

/**
 * Selects the project with the given URL ID, redirecting to the
 * all projects dashboard if no such project exists.
 */
async function selectProject(urlId: string): Promise<void> {
    const goToDashboard = () => {
        const path = ROUTES.Projects.path;
        router.push(path);
    };

    if (urlId.length < MINIMUM_URL_ID_LENGTH) {
        goToDashboard();
        return;
    }

    let projects: Project[];
    try {
        projects = await projectsStore.getProjects();
    } catch {
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
    bucketsStore.clearS3Data();
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

    await selectProject(route.params.id as string);

    try {
        if (!agStore.state.accessGrantsWebWorker) await start();
    } catch (error) {
        notify.error('Unable to set access grants wizard. You may be able to fix this by doing a hard-refresh or clearing your cache.', AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
        // We do this in case user goes to DevTools to check if anything is there.
        // This also might be useful for us since we improve error handling.
        console.error(error.message);
    }

    isLoading.value = false;
});
</script>

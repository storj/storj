// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app>
        <app-bar />
        <default-view />
    </v-app>
</template>

<script setup lang="ts">
import { onBeforeMount } from 'vue';
import { useRouter } from 'vue-router';

import AppBar from './AppBar.vue';
import DefaultView from './DefaultView.vue';

import { RouteConfig } from '@/types/router';
import { Project } from '@/types/projects';
import { useBillingStore } from '@/store/modules/billingStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useABTestingStore } from '@/store/modules/abTestingStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { LocalData } from '@/utils/localData';

const router = useRouter();

const billingStore = useBillingStore();
const usersStore = useUsersStore();
const abTestingStore = useABTestingStore();
const projectsStore = useProjectsStore();

/**
 * Stores project to vuex store and browser's local storage.
 * @param projectID - project id string
 */
function storeProject(projectID: string): void {
    projectsStore.selectProject(projectID);
    LocalData.setSelectedProjectId(projectID);
}

/**
 * Checks if stored project is in fetched projects array and selects it.
 * Selects first fetched project if check is not successful.
 * @param fetchedProjects - fetched projects array
 */
function selectProject(fetchedProjects: Project[]): void {
    const storedProjectID = LocalData.getSelectedProjectId();
    const isProjectInFetchedProjects = fetchedProjects.some(project => project.id === storedProjectID);
    if (storedProjectID && isProjectInFetchedProjects) {
        storeProject(storedProjectID);

        return;
    }

    // Length of fetchedProjects array is checked before selectProject() function call.
    storeProject(fetchedProjects[0].id);
}

/**
 * Lifecycle hook after initial render.
 * Pre fetches user`s and project information.
 */
onBeforeMount(async () => {
    try {
        await Promise.all([
            usersStore.getUser(),
            abTestingStore.fetchValues(),
            usersStore.getSettings(),
        ]);
    } catch (error) {
        setTimeout(async () => await router.push(RouteConfig.Login.path), 1000);

        return;
    }

    try {
        await billingStore.setupAccount();
    } catch (error) { /* empty */ }

    try {
        await billingStore.getCreditCards();
    } catch (error) { /* empty */ }

    let projects: Project[] = [];

    try {
        projects = await projectsStore.getProjects();
    } catch (error) {
        return;
    }

    if (projects.length) {
        selectProject(projects);
    }
});
</script>

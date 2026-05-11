// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <app-shell :is-loading="isLoading">
        <template #nav>
            <ProjectNav />
        </template>
    </app-shell>
</template>

<script setup lang="ts">
import { onBeforeMount, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import type { Project } from '@/types/projects';
import { MINIMUM_URL_ID_LENGTH, useProjectsStore } from '@/store/modules/projectsStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { ROUTES } from '@/router';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useAccessGrantWorker } from '@/composables/useAccessGrantWorker';

import ProjectNav from '@/layouts/project/ProjectNav.vue';
import AppShell from '@/layouts/shared/AppShell.vue';

const router = useRouter();
const route = useRoute();
const notify = useNotify();
const { start } = useAccessGrantWorker();

const bucketsStore = useBucketsStore();
const projectsStore = useProjectsStore();
const agStore = useAccessGrantsStore();
const isLoading = ref<boolean>(true);

/**
 * Selects the project with the given URL ID, redirecting to the
 * all projects dashboard if no such project exists.
 */
async function selectProject(urlId: string): Promise<void> {
    if (urlId.length < MINIMUM_URL_ID_LENGTH) {
        await router.push(ROUTES.Projects.path);
        return;
    }

    let project: Project | undefined = findProject(projectsStore.state.projects, urlId);
    if (project) {
        projectsStore.selectProject(project.id);
        return;
    }

    let projects: Project[];
    try {
        projects = await projectsStore.getProjects();
    } catch {
        await router.push(ROUTES.Projects.path);
        return;
    }

    project = findProject(projects, urlId);
    if (!project) {
        await router.push(ROUTES.Projects.path);
        return;
    }

    projectsStore.selectProject(project.id);
}

function findProject(projects: Project[], urlId: string): Project | undefined {
    return projects.find(p => {
        let prefixEnd = 0;
        while (
            p.urlId[prefixEnd] === urlId[prefixEnd]
            && prefixEnd < p.urlId.length
            && prefixEnd < urlId.length
        ) prefixEnd++;
        return prefixEnd === p.urlId.length || prefixEnd === urlId.length;
    });
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

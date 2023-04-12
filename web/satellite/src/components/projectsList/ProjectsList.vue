// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="projects-list">
        <div class="projects-list__title-area">
            <h2 class="projects-list__title-area__title" aria-roledescription="title">Projects</h2>
            <VButton
                label="Create Project +"
                width="203px"
                height="44px"
                :on-press="onCreateClick"
                :is-disabled="areProjectsFetching"
            />
        </div>
        <VLoader
            v-if="areProjectsFetching"
            width="100px"
            height="100px"
            class="projects-loader"
        />
        <v-table
            v-if="projectsPage.projects.length && !areProjectsFetching"
            class="projects-list-items"
            :limit="projectsPage.limit"
            :total-page-count="projectsPage.pageCount"
            items-label="projects"
            :on-page-click-callback="onPageClick"
            :total-items-count="projectsPage.totalCount"
        >
            <template #head>
                <th class="sort-header-container__name-item align-left">Name</th>
                <th class="ort-header-container__users-item align-left"># Users</th>
                <th class="sort-header-container__date-item align-left">Date Added</th>
            </template>
            <template #body>
                <ProjectsListItem
                    v-for="(project, key) in projectsPage.projects"
                    :key="key"
                    :item-data="project"
                    :on-click="() => onProjectSelected(project)"
                />
            </template>
        </v-table>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { RouteConfig } from '@/router';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { Project, ProjectsPage } from '@/types/projects';
import { LocalData } from '@/utils/localData';
import { AnalyticsHttpApi } from '@/api/analytics';
import { User } from '@/types/users';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useNotify, useRouter, useStore } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useAppStore } from '@/store/modules/appStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';

import ProjectsListItem from '@/components/projectsList/ProjectsListItem.vue';
import VTable from '@/components/common/VTable.vue';
import VLoader from '@/components/common/VLoader.vue';
import VButton from '@/components/common/VButton.vue';

const {
    FETCH_OWNED,
} = PROJECTS_ACTIONS;

const FIRST_PAGE = 1;
const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const agStore = useAccessGrantsStore();
const pmStore = useProjectMembersStore();
const billingStore = useBillingStore();
const usersStore = useUsersStore();
const store = useStore();
const notify = useNotify();
const router = useRouter();

const currentPageNumber = ref<number>(1);
const isLoading = ref<boolean>(false);
const areProjectsFetching = ref<boolean>(true);

/**
 * Returns projects page from store.
 */
const projectsPage = computed((): ProjectsPage => {
    return store.state.projectsModule.page;
});

/**
 * Fetches owned projects page page by clicked page number.
 * @param page
 */
async function onPageClick(page: number): Promise<void> {
    currentPageNumber.value = page;
    try {
        await store.dispatch(FETCH_OWNED, currentPageNumber.value);
    } catch (error) {
        await notify.error(`Unable to fetch owned projects. ${error.message}`, AnalyticsErrorEventSource.PROJECTS_LIST);
    }
}

/**
 * Redirects to create project page.
 */
function onCreateClick(): void {
    analytics.eventTriggered(AnalyticsEvent.NEW_PROJECT_CLICKED);

    const user: User = usersStore.state.user;
    const ownProjectsCount: number = store.getters.projectsCount(user.id);

    if (!user.paidTier && user.projectLimit === ownProjectsCount) {
        appStore.updateActiveModal(MODALS.createProjectPrompt);
    } else {
        analytics.pageVisit(RouteConfig.CreateProject.path);
        appStore.updateActiveModal(MODALS.createProject);
    }
}

/**
 * Fetches all project related information.
 * @param project
 */
async function onProjectSelected(project: Project): Promise<void> {
    if (isLoading.value) return;

    isLoading.value = true;

    const projectID = project.id;
    await store.dispatch(PROJECTS_ACTIONS.SELECT, projectID);
    LocalData.setSelectedProjectId(projectID);
    pmStore.setSearchQuery('');

    try {
        const projectID = store.getters.selectedProject.id;

        await Promise.all([
            billingStore.getProjectUsageAndChargesCurrentRollup(),
            pmStore.getProjectMembers(FIRST_PAGE, projectID),
            agStore.getAccessGrants(FIRST_PAGE, projectID),
            bucketsStore.getBuckets(FIRST_PAGE, projectID),
            store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, projectID),
        ]);

        analytics.pageVisit(RouteConfig.EditProjectDetails.path);
        await router.push(RouteConfig.EditProjectDetails.path);
    } catch (error) {
        await notify.error(`Unable to select project. ${error.message}`, AnalyticsErrorEventSource.PROJECTS_LIST);
    }

    isLoading.value = false;
}

/**
 * Lifecycle hook after initial render where list of existing owned projects is fetched.
 */
onMounted(async () => {
    try {
        await store.dispatch(FETCH_OWNED, currentPageNumber.value);

        areProjectsFetching.value = false;
    } catch (error) {
        await notify.error(`Unable to fetch owned projects. ${error.message}`, AnalyticsErrorEventSource.PROJECTS_LIST);
    }
});
</script>

<style lang="scss">
    .projects-list {
        padding: 40px 30px 55px;
        height: calc(100% - 95px);
        font-family: 'font_regular', sans-serif;

        &__title-area {
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-top: 10px;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 22px;
                line-height: 27px;
                color: #263549;
                margin: 10px 0 0;
            }
        }

        .projects-list-items {
            margin-top: 40px;
        }
    }

    .projects-loader {
        margin-top: 50px;
    }
</style>

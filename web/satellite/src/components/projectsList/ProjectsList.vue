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
            :items="projectsPage.projects"
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
                    :on-click="onProjectSelected"
                />
            </template>
        </v-table>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { Project, ProjectsPage } from '@/types/projects';
import { PM_ACTIONS } from '@/utils/constants/actionNames';
import { LocalData } from '@/utils/localData';
import { AnalyticsHttpApi } from '@/api/analytics';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { User } from '@/types/users';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

import ProjectsListItem from '@/components/projectsList/ProjectsListItem.vue';
import VTable from '@/components/common/VTable.vue';
import VLoader from '@/components/common/VLoader.vue';
import VButton from '@/components/common/VButton.vue';

const {
    FETCH_OWNED,
} = PROJECTS_ACTIONS;

// @vue/component
@Component({
    components: {
        ProjectsListItem,
        VButton,
        VLoader,
        VTable,
    },
})
export default class Projects extends Vue {
    private currentPageNumber = 1;
    private FIRST_PAGE = 1;
    private isLoading = false;

    public areProjectsFetching = true;

    public readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Lifecycle hook after initial render where list of existing owned projects is fetched.
     */
    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch(FETCH_OWNED, this.currentPageNumber);

            this.areProjectsFetching = false;
        } catch (error) {
            await this.$notify.error(`Unable to fetch owned projects. ${error.message}`);
        }
    }

    /**
     * Fetches owned projects page page by clicked page number.
     * @param page
     */
    public async onPageClick(page: number): Promise<void> {
        this.currentPageNumber = page;
        try {
            await this.$store.dispatch(FETCH_OWNED, this.currentPageNumber);
        } catch (error) {
            await this.$notify.error(`Unable to fetch owned projects. ${error.message}`);
        }
    }

    /**
     * Redirects to create project page.
     */
    public onCreateClick(): void {
        this.analytics.eventTriggered(AnalyticsEvent.NEW_PROJECT_CLICKED);

        const user: User = this.$store.getters.user;
        const ownProjectsCount: number = this.$store.getters.projectsCount;

        if (!user.paidTier && user.projectLimit === ownProjectsCount) {
            this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_CREATE_PROJECT_PROMPT_POPUP);
        } else {
            this.analytics.pageVisit(RouteConfig.CreateProject.path);
            this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_CREATE_PROJECT_POPUP);
        }
    }

    /**
     * Fetches all project related information.
     * @param project
     */
    public async onProjectSelected(project: Project): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        const projectID = project.id;
        await this.$store.dispatch(PROJECTS_ACTIONS.SELECT, projectID);
        LocalData.setSelectedProjectId(projectID);
        await this.$store.dispatch(PM_ACTIONS.SET_SEARCH_QUERY, '');

        try {
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);
            await this.$store.dispatch(PM_ACTIONS.FETCH, this.FIRST_PAGE);
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.FETCH, this.FIRST_PAGE);
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH, this.FIRST_PAGE);
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, this.$store.getters.selectedProject.id);

            this.analytics.pageVisit(RouteConfig.EditProjectDetails.path);
            await this.$router.push(RouteConfig.EditProjectDetails.path);
        } catch (error) {
            await this.$notify.error(`Unable to select project. ${error.message}`);
        }

        this.isLoading = false;
    }

    /**
     * Returns ProjectsList item component.
     */
    public get itemComponent(): typeof ProjectsListItem {
        return ProjectsListItem;
    }

    /**
     * Returns projects page from store.
     */
    public get projectsPage(): ProjectsPage {
        return this.$store.state.projectsModule.page;
    }

}
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

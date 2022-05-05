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
        <VLoader v-if="areProjectsFetching" width="100px" height="100px" class="projects-loader" />
        <div v-if="projectsPage.projects.length && !areProjectsFetching" class="projects-list-items">
            <SortProjectsListHeader />
            <div class="projects-list-items__content">
                <VList
                    :data-set="projectsPage.projects"
                    :item-component="itemComponent"
                    :on-item-click="onProjectSelected"
                />
            </div>
            <div v-if="projectsPage.pageCount > 1" class="projects-list-items__pagination-area">
                <VPagination
                    :total-page-count="projectsPage.pageCount"
                    :on-page-click-callback="onPageClick"
                />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';
import VList from '@/components/common/VList.vue';
import VLoader from '@/components/common/VLoader.vue';
import VPagination from '@/components/common/VPagination.vue';
import ProjectsListItem from '@/components/projectsList/ProjectsListItem.vue';
import SortProjectsListHeader from '@/components/projectsList/SortProjectsListHeader.vue';

import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { Project, ProjectsPage } from '@/types/projects';
import { PM_ACTIONS } from '@/utils/constants/actionNames';
import { LocalData } from '@/utils/localData';

const {
    FETCH_OWNED,
} = PROJECTS_ACTIONS;

// @vue/component
@Component({
    components: {
        SortProjectsListHeader,
        VButton,
        VList,
        VPagination,
        VLoader,
    },
})
export default class Projects extends Vue {
    private currentPageNumber = 1;
    private FIRST_PAGE = 1;

    public areProjectsFetching = true;

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
        this.$router.push(RouteConfig.CreateProject.path);
    }

    /**
     * Fetches all project related information.
     * @param project
     */
    public async onProjectSelected(project: Project): Promise<void> {
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

            if (this.isNewNavStructure) {
                await this.$router.push(RouteConfig.EditProjectDetails.path);

                return;
            }

            await this.$router.push(RouteConfig.ProjectDashboard.path);
        } catch (error) {
            await this.$notify.error(`Unable to select project. ${error.message}`);
        }
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

    /**
     * Indicates if new navigation structure is used.
     */
    public get isNewNavStructure(): boolean {
        return this.$store.state.appStateModule.isNewNavStructure;
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

            &__content {
                background-color: #fff;
                display: flex;
                flex-direction: column;
                width: calc(100% - 32px);
                justify-content: flex-start;
                padding: 16px;
                border-radius: 0 0 8px 8px;
            }
        }
    }

    .projects-loader {
        margin-top: 50px;
    }
</style>

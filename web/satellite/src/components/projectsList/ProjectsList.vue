// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="projects-list">
        <div class="projects-list__title-area">
            <h2 class="projects-list__title-area__title">Projects</h2>
            <VButton
                label="Create Project +"
                width="203px"
                height="44px"
                :on-press="onCreateClick"
            />
        </div>

        <div class="projects-list__title-area__right">

        </div>
        <div class="projects-list-items" v-if="projectsPage.projects.length">
            <SortProjectsListHeader />
            <div class="projects-list-items__content">
                <VList
                    :data-set="projectsPage.projects"
                    :item-component="itemComponent"
                />
            </div>
            <div class="projects-list-items__pagination-area">
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
import { ProjectsApiGql } from '@/api/projects';
import { ProjectsCursor, ProjectsPage, Project } from '@/types/projects';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { RouteConfig } from '@/router';

import ProjectsListItem from '@/components/projectsList/ProjectsListItem.vue'
import SortProjectsListHeader from '@/components/projectsList/SortProjectsListHeader.vue'
import VButton from '@/components/common/VButton.vue';
import VList from '@/components/common/VList.vue';
import VPagination from '@/components/common/VPagination.vue';

const {
    FETCH_OWNED,
} = PROJECTS_ACTIONS;

@Component({
    components: {
        SortProjectsListHeader,
        VButton,
        VList,
        VPagination,
    },
})
export default class Projects extends Vue {

    private currentPageNumber: number = 1;

    private projectsApi: ProjectsApiGql = new ProjectsApiGql();

    private currentProjectsPage: ProjectsPage = new ProjectsPage();

    /**
     * Lifecycle hook after initial render where list of existing access grants is fetched.
     */
    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch(FETCH_OWNED, this.currentPageNumber);
        } catch(error) {
            await this.$notify.error(`Unable to fetch owned projects. ${error.message}`);
        }
    }

    public async onPageClick(page: number): Promise<void> {
        this.currentPageNumber = page;
        try {
            await this.$store.dispatch(FETCH_OWNED, this.currentPageNumber);
        } catch(error) {
            await this.$notify.error(`Unable to fetch owned projects. ${error.message}`);
        }
    }

    public get itemComponent() {
        return ProjectsListItem;
    }

    public onCreateClick(): void {
        this.$router.push(RouteConfig.CreateProject.path);
    }

    public get projectsPage(): ProjectsPage {
        return this.$store.state.projectsModule.page;
    }



}
</script>

<style lang="scss">
    .projects-list {
        position: relative;
        padding: 40px 30px 55px 30px;
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
                margin: 0;
            }
        }

        .projects-list-items {
            position: relative;

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
</style>

// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="projects-list">
        <div class="projects-list__title-area">
            <h2 class="projects-list__title-area__title">Projects</h2>
        </div>
        <div class="projects-list__title-area__right" v-if="currentProjectsPage.projects">
            <VButton
                label="Create Project +"
                width="203px"
                height="44px"
                :on-press="onCreateClick"
            />
        </div>
        <div class="projects-list-items" v-if="currentProjectsPage.projects">
            <SortProjectsListHeader />
            <div class="projects-list-items__content">
                <VList
                    :data-set="projects"
                    :item-component="itemComponent"
                />
            </div>
            <div class="projects-list-items__pagination-area">
                <VPagination
                    :total-page-count="currentProjectsPage.pageCount"
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
import { RouteConfig } from '@/router';

import ProjectsListItem from '@/components/projectsList/ProjectsListItem.vue'
import SortProjectsListHeader from '@/components/projectsList/SortProjectsListHeader.vue'
import VButton from '@/components/common/VButton.vue';
import VList from '@/components/common/VList.vue';
import VPagination from '@/components/common/VPagination.vue';

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
    * Component initialization.
    */
    public mounted() {
        this.queryProjectsApi();
    }

    /**
    * Determines whet
    her test banner should be displayed.
    */
    public async queryProjectsApi(): Promise<void> {
        const response = await this.projectsApi.getOwnedProjects(new ProjectsCursor(2, this.currentPageNumber));
        console.log("RESPN:", response);
        this.currentProjectsPage = response;
    }

    public async onPageClick(page: number): Promise<void> {
        // try {
        //     await this.$store.dispatch(FETCH, page);
        // } catch (error) {
        //     await this.$notify.error(`Unable to fetch buckets: ${error.message}`);
        // }
        this.currentPageNumber = page;
        this.queryProjectsApi();
        console.log("PAGE CLICK");
    }

    public get itemComponent() {
        return ProjectsListItem;
    }

    public get projects(): Project[] {
        console.log("PROJETS:", this.currentProjectsPage.projects)
        return this.currentProjectsPage.projects;
    }

    public onCreateClick(): void {
        this.$router.push(RouteConfig.CreateProject.path);
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
            float: left;
            position: relative;
            top: 10px;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 22px;
                line-height: 27px;
                color: #263549;
                margin: 0;
            }

            &__right {
                float: right;
                margin: 0 0 20px 0;

                .container {
                    background: #BBBEC2;
                }
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

// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="projects-list">
        <div class="projects-list__title-area">
            <h2 class="projects-list__title-area__title">Projects</h2>
        </div>
        <div class="projects-list-items" v-if="currentProjectsPage.projects">
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

import ProjectsListItem from "@/components/projectsList/ProjectsListItem.vue"
import VList from '@/components/common/VList.vue';
import VPagination from '@/components/common/VPagination.vue';

@Component({
    components: {
        VList,
        VPagination,
    },
})
export default class Projects extends Vue {

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
        const response = await this.projectsApi.getOwnedProjects(new ProjectsCursor(5, 1));
        console.log("RESPN:", response);
        this.currentProjectsPage = response;
    }

    public async onPageClick(page: number): Promise<void> {
        // try {
        //     await this.$store.dispatch(FETCH, page);
        // } catch (error) {
        //     await this.$notify.error(`Unable to fetch buckets: ${error.message}`);
        // }
        console.log("PAGE CLICK")
    }

    public get itemComponent() {
        return ProjectsListItem;
    }

    public get projects(): Project[] {
        console.log("PROJETS:", this.currentProjectsPage.projects)
        return this.currentProjectsPage.projects;
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

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 22px;
                line-height: 27px;
                color: #263549;
                margin: 0;
            }
        }
    }
</style>

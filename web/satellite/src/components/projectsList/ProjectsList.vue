// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="projects-list">
        <div class="projects-list__title-area">
            <h2 class="projects-list__title-area__title">Projects</h2>
        </div>
    </div>
</template>

<script>
import { Component, Vue } from 'vue-property-decorator';
import { ProjectsApiGql } from '@api/projects';
import { ProjectsCursor } from '@types/projects';

@Component
export default class Projects extends Vue {

    private projects: ProjectsApiGql = new ProjectsApiGql();

    /**
    * Component initialization.
    */
    public mounted() {
        this.queryProjectsApi();
    }

    /**
    * Determines whether test banner should be displayed.
    */
    public async queryProjectsApi(): Promise<void> {
        const response = await this.projects.getOwnedProjects(new ProjectsCursor(5, 1));
        console.log("RESP:", response)
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

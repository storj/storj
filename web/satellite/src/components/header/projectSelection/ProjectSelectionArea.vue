// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-selection-container" id="projectDropdownButton">
        <p class="project-selection-container__no-projects-text" v-if="!hasProjects">You have no projects</p>
        <div class="project-selection-toggle-container" @click="toggleSelection" v-if="hasProjects">
            <h1>{{name}}</h1>
            <div class="project-selection-toggle-container__expander-area">
                <img v-if="!isDropdownShown" src="../../../../static/images/register/BlueExpand.svg" alt="expand project list" />
                <img v-if="isDropdownShown" src="../../../../static/images/register/BlueHide.svg" alt="hide project list" />
            </div>
        </div>
        <ProjectSelectionDropdown v-if="isDropdownShown"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { Project } from '@/types/projects';
import { APP_STATE_ACTIONS, NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';

import ProjectSelectionDropdown from './ProjectSelectionDropdown.vue';

@Component({
    components: {
        ProjectSelectionDropdown,
    },
})
export default class ProjectSelectionArea extends Vue {
    public async toggleSelection(): Promise<void> {
        try {
            await this.$store.dispatch(PROJECTS_ACTIONS.FETCH);
        } catch (e) {
            await this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, e.message);
        }

        await this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_PROJECTS);
    }

    public get name(): string {
        const selectedProject: Project = this.$store.state.projectsModule.selectedProject;

        return selectedProject.id ? selectedProject.name : 'Choose project';
    }

    public get isDropdownShown(): boolean {
        return this.$store.state.appStateModule.appState.isProjectsDropdownShown;
    }

    public get hasProjects(): boolean {
        return !!this.$store.state.projectsModule.projects.length;
    }
}
</script>

<style scoped lang="scss">
    .project-selection-container {
        position: relative;
        background-color: #FFFFFF;
        cursor: pointer;

        &__no-projects-text {
            font-family: 'font_medium';
            font-size: 16px;
            line-height: 23px;
            color: #354049;
            opacity: 0.7;
            cursor: default !important;
        }

        h1 {
            font-family: 'font_medium';
            font-size: 16px;
            line-height: 23px;
            color: #354049;
        }

        &:hover {

            h1 {
                opacity: 0.7;
            }
        }
    }

    .project-selection-toggle-container {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: flex-start;
        width: 100%;
        height: 50px;

        h1 {
            transition: opacity .2s ease-in-out;
        }

        &__expander-area {
            margin-left: 12px;
            display: flex;
            align-items: center;
            justify-content: center;
            width: 28px;
            height: 28px;
        }
    }

    @media screen and (max-width: 1024px) {
        .project-selection-container {
            margin-right: 30px;
            padding-right: 10px;
        }

        .project-selection-toggle-container {
            justify-content: space-between;
            margin-left: 10px;
        }
    }
</style>

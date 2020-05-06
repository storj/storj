// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-selection-container" id="projectDropdownButton">
        <p class="project-selection-container__no-projects-text" v-if="!hasProjects">You have no projects</p>
        <div
            class="project-selection-toggle-container"
            :class="{ default: isOnboardingTour }"
            @click="toggleSelection"
            v-if="hasProjects"
        >
            <p class="project-selection-toggle-container__common" :class="{ default: isOnboardingTour }">Project:</p>
            <h1 class="project-selection-toggle-container__name">{{name}}</h1>
            <div class="project-selection-toggle-container__expander-area" v-if="!isOnboardingTour">
                <ExpandIcon
                    v-if="!isDropdownShown"
                    alt="Arrow down (expand)"
                />
                <HideIcon
                    v-if="isDropdownShown"
                    alt="Arrow up (hide)"
                />
            </div>
        </div>
        <ProjectSelectionDropdown v-if="isDropdownShown"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ExpandIcon from '@/../static/images/common/BlueExpand.svg';
import HideIcon from '@/../static/images/common/BlueHide.svg';

import { RouteConfig } from '@/router';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { Project } from '@/types/projects';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

import ProjectSelectionDropdown from './ProjectSelectionDropdown.vue';

@Component({
    components: {
        ProjectSelectionDropdown,
        ExpandIcon,
        HideIcon,
    },
})
export default class ProjectSelectionArea extends Vue {
    private isLoading: boolean = false;

    /**
     * Fetches projects related information and than toggles selection popup.
     */
    public async toggleSelection(): Promise<void> {
        if (this.isLoading || this.isOnboardingTour) return;

        this.isLoading = true;

        try {
            await this.$store.dispatch(PROJECTS_ACTIONS.FETCH);
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, this.$store.getters.selectedProject.id);
        } catch (error) {
            await this.$notify.error(error.message);
            this.isLoading = false;
        }

        await this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_PROJECTS);
        this.isLoading = false;
    }

    /**
     * Return selected project name if it is, if not returns default label.
     */
    public get name(): string {
        const selectedProject: Project = this.$store.state.projectsModule.selectedProject;

        return selectedProject.id ? selectedProject.name : 'Choose project';
    }

    /**
     * Indicates if project selection dropdown should be rendered.
     */
    public get isDropdownShown(): boolean {
        return this.$store.state.appStateModule.appState.isProjectsDropdownShown;
    }

    /**
     * Indicates if user has projects.
     */
    public get hasProjects(): boolean {
        return !!this.$store.state.projectsModule.projects.length;
    }

    /**
     * Indicates if current route is onboarding tour.
     */
    public get isOnboardingTour(): boolean {
        return this.$route.name === RouteConfig.OnboardingTour.name;
    }
}
</script>

<style scoped lang="scss">
    .project-selection-container {
        position: relative;
        background-color: #fff;
        cursor: pointer;

        &__no-projects-text {
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            line-height: 23px;
            color: #354049;
            opacity: 0.7;
            cursor: default !important;
        }
    }

    .project-selection-toggle-container {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: flex-start;
        width: 100%;
        height: 50px;

        &__common {
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            line-height: 23px;
            color: rgba(56, 75, 101, 0.4);
            opacity: 0.7;
            cursor: pointer;
            margin-right: 5px;
        }

        &__name {
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            line-height: 23px;
            color: #354049;
            transition: opacity 0.2s ease-in-out;
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

    .default {
        cursor: default;
    }

    @media screen and (max-width: 1280px) {

        .project-selection-container {
            margin-right: 30px;
            padding-right: 10px;
        }

        .project-selection-toggle-container {
            justify-content: space-between;
            margin-left: 10px;

            &__common {
                display: none;
            }
        }
    }
</style>

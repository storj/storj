// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-selection" :class="{ default: !hasProjects }">
        <p class="project-selection__no-projects-text" v-if="!hasProjects">You have no projects</p>
        <div
            class="project-selection__toggle-container"
            :class="{ default: isOnboardingTour }"
            @click.stop="toggleSelection"
            v-if="hasProjects"
        >
            <h1 class="project-selection__toggle-container__name">{{name}}</h1>
            <div class="project-selection__toggle-container__expander-area" v-if="!isOnboardingTour">
                <ExpandIcon
                    v-if="!isDropdownShown"
                    alt="Arrow down (expand)"
                />
                <HideIcon
                    v-if="isDropdownShown"
                    alt="Arrow up (hide)"
                />
            </div>
            <ProjectDropdown
                v-show="isDropdownShown"
                @close="closeDropdown"
                v-click-outside="closeDropdown"
            />
        </div>
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
import { MetaUtils } from '@/utils/meta';

import ProjectDropdown from './ProjectDropdown.vue';

@Component({
    components: {
        ProjectDropdown,
        ExpandIcon,
        HideIcon,
    },
})
export default class ProjectSelection extends Vue {
    private isLoading: boolean = false;
    public isDropdownShown: boolean = false;

    /**
     * Life cycle hook before initial render.
     * Toggles new project button visibility depending on user reaching project count limit or having payment method.
     */
    public beforeMount(): void {
        if (this.isProjectLimitReached || !this.$store.getters.canUserCreateFirstProject) {
            this.$store.dispatch(APP_STATE_ACTIONS.HIDE_CREATE_PROJECT_BUTTON);

            return;
        }

        this.$store.dispatch(APP_STATE_ACTIONS.SHOW_CREATE_PROJECT_BUTTON);
    }

    /**
     * Indicates if new project creation button is shown.
     */
    public get isButtonShown(): boolean {
        return this.$store.state.appStateModule.appState.isCreateProjectButtonShown;
    }

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

        this.toggleDropdown();
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
     * Indicates if user has projects.
     */
    public get hasProjects(): boolean {
        return this.$store.state.projectsModule.projects.length > 0;
    }

    /**
     * Indicates if current route is onboarding tour.
     */
    public get isOnboardingTour(): boolean {
        return this.$route.name === RouteConfig.OnboardingTour.name;
    }

    /**
     * Toggles project dropdown visibility.
     */
    public toggleDropdown(): void {
        this.isDropdownShown = !this.isDropdownShown;
    }

    /**
     * Closes project dropdown.
     */
    public closeDropdown(): void {
        this.isDropdownShown = false;
    }

    /**
     * Indicates if project count limit is reached.
     */
    private get isProjectLimitReached(): boolean {
        const defaultProjectLimit: number = parseInt(MetaUtils.getMetaContent('default-project-limit'));

        return this.$store.getters.userProjectsCount >= defaultProjectLimit;
    }
}
</script>

<style scoped lang="scss">
    .project-selection {
        background-color: #fff;
        cursor: pointer;
        margin-right: 20px;

        &__no-projects-text {
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            line-height: 23px;
            color: #354049;
            opacity: 0.7;
            cursor: default !important;
        }

        &__toggle-container {
            position: relative;
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
                word-break: break-all;
            }

            &__expander-area {
                margin-left: 11px;
                display: flex;
                align-items: center;
                justify-content: center;
                width: 28px;
                height: 28px;
            }
        }
    }

    .default {
        cursor: default;
    }

    @media screen and (max-width: 1280px) {

        .project-selection {
            margin-right: 30px;

            &__toggle-container {
                justify-content: space-between;
                padding-left: 10px;

                &__common {
                    display: none;
                }
            }
        }
    }
</style>

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-selection" :class="{ disabled: isOnboardingTour, active: isDropdownShown }">
        <div
            class="project-selection__toggle-container"
            @click.stop="toggleSelection"
        >
            <h1 class="project-selection__toggle-container__name" :class="{ white: isDropdownShown }">Projects</h1>
            <ExpandIcon
                class="project-selection__toggle-container__expand-icon"
                :class="{ expanded: isDropdownShown }"
                alt="Arrow down (expand)"
            />
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

import ExpandIcon from '@/../static/images/common/BlackArrowExpand.svg';

import { RouteConfig } from '@/router';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { MetaUtils } from '@/utils/meta';

import ProjectDropdown from './ProjectDropdown.vue';

@Component({
    components: {
        ProjectDropdown,
        ExpandIcon,
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
        min-width: 130px;

        &__toggle-container {
            position: relative;
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 0 16px;
            width: calc(100% - 32px);
            height: 36px;

            &__name {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 23px;
                color: #354049;
                transition: opacity 0.2s ease-in-out;
                word-break: unset;
                margin: 0;
            }

            &__expand-icon {
                margin-left: 15px;
            }
        }
    }

    .disabled {
        opacity: 0.5;
        pointer-events: none;
        cursor: default;
    }

    .expanded {

        .black-arrow-expand-path {
            fill: #fff;
        }
    }

    .active {
        background: #2582ff;
        border-radius: 6px;
    }

    .white {
        color: #fff;
    }

    @media screen and (max-width: 1280px) {

        .project-selection {
            margin-right: 30px;

            &__toggle-container {
                justify-content: space-between;
                padding-left: 10px;
            }
        }
    }
</style>

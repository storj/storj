// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="new-project-container">
        <div
            v-if="isButtonShown"
            class="new-project-button-container"
            @click="toggleSelection"
            id="newProjectButton"
        >
            <h1 class="new-project-button-container__label">+ Create Project</h1>
        </div>
        <NewProjectPopup v-if="isPopupShown"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VInfo from '@/components/common/VInfo.vue';
import NewProjectPopup from '@/components/project/NewProjectPopup.vue';

import { RouteConfig } from '@/router';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { ProjectOwning } from '@/utils/projectOwning';

@Component({
    components: {
        VInfo,
        NewProjectPopup,
    },
})
export default class NewProjectArea extends Vue {
    // TODO: temporary solution. Remove when user will be able to create more then one project
    /**
     * Life cycle hook before initial render.
     * Toggles new project button visibility depending on user having his own project or payment method.
     */
    public beforeMount(): void {
        if (this.userHasOwnProject || !this.$store.getters.canUserCreateFirstProject) {
            this.$store.dispatch(APP_STATE_ACTIONS.HIDE_CREATE_PROJECT_BUTTON);

            return;
        }

        this.$store.dispatch(APP_STATE_ACTIONS.SHOW_CREATE_PROJECT_BUTTON);
    }

    /**
     * Life cycle hook after initial render.
     * Hides new project button visibility if user is on onboarding tour.
     */
    public mounted(): void {
        if (this.isOnboardingTour) {
            this.$store.dispatch(APP_STATE_ACTIONS.HIDE_CREATE_PROJECT_BUTTON);
        }
    }

    /**
     * Opens new project creation popup.
     */
    public toggleSelection(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_NEW_PROJ);
    }

    /**
     * Indicates if new project creation popup should be rendered.
     */
    public get isPopupShown(): boolean {
        return this.$store.state.appStateModule.appState.isNewProjectPopupShown;
    }

    /**
     * Indicates if new project creation button is shown.
     */
    public get isButtonShown(): boolean {
        return this.$store.state.appStateModule.appState.isCreateProjectButtonShown;
    }

    /**
     * Indicates if current route is onboarding tour.
     */
    public get isOnboardingTour(): boolean {
        return this.$route.name === RouteConfig.OnboardingTour.name;
    }

    /**
     * Indicates if user has own project.
     */
    private get userHasOwnProject(): boolean {
        return new ProjectOwning(this.$store).userHasOwnProject();
    }
}
</script>

<style scoped lang="scss">
    .new-project-container {
        background-color: #fff;
        position: relative;
    }

    .new-project-button-container {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 156px;
        height: 40px;
        border-radius: 6px;
        border: 2px solid #2683ff;
        background-color: transparent;
        cursor: pointer;

        &__label {
            font-family: 'font_medium', sans-serif;
            font-size: 15px;
            line-height: 22px;
            color: #2683ff;
        }

        &:hover {
            background-color: #2683ff;
            border: 2px solid #2683ff;

            .new-project-button-container__label {
                color: white;
            }
        }
    }
</style>

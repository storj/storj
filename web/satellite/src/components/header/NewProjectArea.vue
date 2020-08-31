// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="new-project-container">
        <div
            v-if="isButtonShown && !isOnboardingTour"
            class="new-project-button-container"
            @click="onCreateProjectClick"
            id="newProjectButton"
        >
            <h1 class="new-project-button-container__label">+ Create Project</h1>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VInfo from '@/components/common/VInfo.vue';

import { RouteConfig } from '@/router';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { MetaUtils } from '@/utils/meta';
import { ProjectOwning } from '@/utils/projectOwning';

@Component({
    components: {
        VInfo,
    },
})
export default class NewProjectArea extends Vue {
    /**
     * Life cycle hook before initial render.
     * Toggles new project button visibility depending on user having his own project or payment method.
     */
    public beforeMount(): void {
        const defaultProjectLimit: number = parseInt(MetaUtils.getMetaContent('default-project-limit'));
        if (this.usersProjectsAmount >= defaultProjectLimit || !this.$store.getters.canUserCreateFirstProject) {
            this.$store.dispatch(APP_STATE_ACTIONS.HIDE_CREATE_PROJECT_BUTTON);

            return;
        }

        this.$store.dispatch(APP_STATE_ACTIONS.SHOW_CREATE_PROJECT_BUTTON);
    }

    /**
     * Redirects to create project page.
     */
    public onCreateProjectClick(): void {
        this.$router.push(RouteConfig.CreateProject.path);
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
     * Returns user's projects amount.
     */
    private get usersProjectsAmount(): number {
        return new ProjectOwning(this.$store).usersProjectsCount();
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

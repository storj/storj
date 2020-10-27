// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="create-project-area">
        <div class="create-project-area__container">
            <img src="@/../static/images/project/createProject.png" alt="create project image">
            <h2 class="create-project-area__title">Create a Project</h2>
            <HeaderedInput
                label="Project Name"
                additional-label="Up To 20 Characters"
                placeholder="Enter Project Name"
                class="full-input"
                width="100%"
                is-limit-shown="true"
                :current-limit="projectName.length"
                :max-symbols="20"
                :error="nameError"
                @setData="setProjectName"
            />
            <HeaderedInput
                label="Description"
                placeholder="Enter Project Description"
                additional-label="Optional"
                class="full-input"
                is-multiline="true"
                height="60px"
                width="calc(100% - 42px)"
                is-limit-shown="true"
                :current-limit="description.length"
                :max-symbols="100"
                @setData="setProjectDescription"
            />
            <div class="create-project-area__container__button-container">
                <VButton
                    label="Cancel"
                    width="210px"
                    height="48px"
                    :on-press="onCancelClick"
                    is-transparent="true"
                />
                <VButton
                    label="Create Project +"
                    width="210px"
                    height="48px"
                    :on-press="onCreateProjectClick"
                    :is-disabled="!projectName"
                />
            </div>
            <div class="create-project-area__container__blur" v-if="isLoading">
                <img
                    class="create-project-area__container__blur__loading-image"
                    src="@/../static/images/account/billing/loading.gif"
                    alt="loading gif"
                >
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderedInput from '@/components/common/HeaderedInput.vue';
import VButton from '@/components/common/VButton.vue';

import { RouteConfig } from '@/router';
import { API_KEYS_ACTIONS } from '@/store/modules/apiKeys';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { ProjectFields } from '@/types/projects';
import {
    APP_STATE_ACTIONS,
    PM_ACTIONS,
} from '@/utils/constants/actionNames';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';
import { LocalData } from '@/utils/localData';

@Component({
    components: {
        HeaderedInput,
        VButton,
    },
})
export default class NewProjectPopup extends Vue {
    private description: string = '';
    private createdProjectId: string = '';
    private isLoading: boolean = false;

    public projectName: string = '';
    public nameError: string = '';

    /**
     * Sets project name from input value.
     */
    public setProjectName(value: string): void {
        this.projectName = value;
        this.nameError = '';
    }

    /**
     * Sets project description from input value.
     */
    public setProjectDescription(value: string): void {
        this.description = value;
    }

    /**
     * Redirects to previous route.
     */
    public onCancelClick(): void {
        const PREVIOUS_ROUTE_NUMBER = -1;

        this.$router.go(PREVIOUS_ROUTE_NUMBER);
    }

    /**
     * Creates project and refreshes store.
     */
    public async onCreateProjectClick(): Promise<void> {
        if (this.isLoading) {
            return;
        }

        this.isLoading = true;
        this.projectName = this.projectName.trim();

        const project = new ProjectFields(
            this.projectName,
            this.description,
            this.$store.getters.user.id,
        );

        try {
            project.checkName();
        } catch (error) {
            this.isLoading = false;
            this.nameError = error.message;

            return;
        }

        try {
            const createdProject = await this.$store.dispatch(PROJECTS_ACTIONS.CREATE, project);
            this.createdProjectId = createdProject.id;
            this.$segment.track(SegmentEvent.PROJECT_CREATED, {
                project_id: this.createdProjectId,
            });
        } catch (error) {
            this.isLoading = false;
            await this.$notify.error(error.message);

            return;
        }

        this.selectCreatedProject();

        try {
            await this.fetchProjectMembers();
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, this.createdProjectId);
        } catch (error) {
            await this.$notify.error(`Unable to create project. ${error.message}`);
        }

        this.clearApiKeys();

        this.clearBucketUsage();

        await this.$notify.success('Project created successfully!');

        this.isLoading = false;

        await this.$router.push(RouteConfig.ProjectDashboard.path);
    }

    /**
     * Selects just created project.
     */
    private selectCreatedProject(): void {
        this.$store.dispatch(PROJECTS_ACTIONS.SELECT, this.createdProjectId);
        LocalData.setSelectedProjectId(this.createdProjectId);

        if (this.$store.getters.projectsCount >= this.$store.getters.user.projectLimit) {
            this.$store.dispatch(APP_STATE_ACTIONS.HIDE_CREATE_PROJECT_BUTTON);
        }
    }

    /**
     * Clears project members store and fetches new.
     */
    private async fetchProjectMembers(): Promise<void> {
        await this.$store.dispatch(PM_ACTIONS.CLEAR);
        const fistPage = 1;
        await this.$store.dispatch(PM_ACTIONS.FETCH, fistPage);
    }

    /**
     * Clears api keys store.
     */
    private clearApiKeys(): void {
        this.$store.dispatch(API_KEYS_ACTIONS.CLEAR);
    }

    /**
     * Clears bucket usage store.
     */
    private clearBucketUsage(): void {
        this.$store.dispatch(BUCKET_ACTIONS.CLEAR);
    }
}
</script>

<style scoped lang="scss">
    .full-input {
        width: 100%;
    }

    .create-project-area {
        display: flex;
        align-items: center;
        justify-content: center;
        width: calc(100% - 40px);
        padding: 100px 20px 70px 20px;
        font-family: 'font_regular', sans-serif;

        &__container {
            width: 440px;
            background-color: #fff;
            border-radius: 8px;
            display: flex;
            flex-direction: column;
            align-items: center;
            padding: 70px 50px 55px 50px;
            position: relative;

            &__title {
                font-size: 28px;
                line-height: 34px;
                color: #384b65;
                font-family: 'font_bold', sans-serif;
            }

            &__button-container {
                width: 100%;
                display: flex;
                align-items: center;
                justify-content: space-between;
                margin-top: 30px;
            }

            &__blur {
                position: absolute;
                top: 0;
                left: 0;
                height: 100%;
                width: 100%;
                background-color: rgba(229, 229, 229, 0.2);
                border-radius: 8px;
                z-index: 100;

                &__loading-image {
                    width: 25px;
                    height: 25px;
                    position: absolute;
                    right: 40px;
                    top: 40px;
                }
            }
        }
    }
</style>

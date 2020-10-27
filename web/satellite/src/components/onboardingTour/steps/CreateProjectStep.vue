// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="new-project-step">
        <h1 class="new-project-step__title">Name Your Project</h1>
        <p class="new-project-step__sub-title">
            Projects are where buckets are created for storing data. Within a Project, usage is tracked at the bucket
            level and aggregated for billing.
        </p>
        <div class="new-project-step__container">
            <div class="new-project-step__container__title-area">
                <h2 class="new-project-step__container__title-area__title">Project Details</h2>
                <img
                    v-if="isLoading"
                    class="new-project-step__container__title-area__loading-image"
                    src="@/../static/images/account/billing/loading.gif"
                    alt="loading gif"
                >
            </div>
            <HeaderedInput
                label="Project Name"
                additional-label="Up To 20 Characters"
                placeholder="Enter Project Name"
                class="full-input project-name-input"
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
                is-limit-shown="true"
                :current-limit="description.length"
                :max-symbols="100"
                height="60px"
                width="calc(100% - 42px)"
                @setData="setProjectDescription"
            />
            <div class="new-project-step__container__blur" v-if="isLoading"/>
        </div>
        <VButton
            class="create-project-button"
            width="156px"
            height="48px"
            label="Create Project"
            :on-press="createProjectClick"
            :is-disabled="!projectName"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderedInput from '@/components/common/HeaderedInput.vue';
import VButton from '@/components/common/VButton.vue';

import { API_KEYS_ACTIONS } from '@/store/modules/apiKeys';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { ProjectFields } from '@/types/projects';
import { PM_ACTIONS } from '@/utils/constants/actionNames';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';

@Component({
    components: {
        VButton,
        HeaderedInput,
    },
})
export default class CreateProjectStep extends Vue {
    private description: string = '';

    public projectName: string = '';
    public isLoading: boolean = false;
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
     * Creates project and refreshes store.
     */
    public async createProjectClick(): Promise<void> {
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

        let createdProjectId: string = '';

        try {
            const createdProject = await this.$store.dispatch(PROJECTS_ACTIONS.CREATE, project);
            createdProjectId = createdProject.id;
            this.$segment.track(SegmentEvent.PROJECT_CREATED, {
                project_id: createdProjectId,
            });
        } catch (error) {
            this.isLoading = false;
            await this.$notify.error(error.message);

            return;
        }

        await this.$store.dispatch(PROJECTS_ACTIONS.SELECT, createdProjectId);

        try {
            await this.fetchProjectMembers();
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PAYMENTS_HISTORY);
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_BALANCE);
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, createdProjectId);
        } catch (error) {
            await this.$notify.error(`Unable to create project. ${error.message}`);
        }

        this.clearApiKeys();

        this.clearBucketUsage();

        await this.$notify.success('Project created successfully!');

        this.isLoading = false;

        this.$emit('setApiKeyState');
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
    h1,
    h2,
    p {
        margin: 0;
    }

    .new-project-step {
        font-family: 'font_regular', sans-serif;
        margin-top: 75px;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: space-between;
        padding: 0 200px;

        &__title {
            font-size: 32px;
            line-height: 39px;
            color: #1b2533;
            margin-bottom: 25px;
        }

        &__sub-title {
            font-size: 16px;
            line-height: 19px;
            color: #354049;
            margin-bottom: 35px;
            text-align: center;
            word-break: break-word;
        }

        &__container {
            padding: 50px;
            width: calc(100% - 100px);
            border-radius: 8px;
            background-color: #fff;
            position: relative;
            margin-bottom: 30px;

            &__title-area {
                display: flex;
                align-items: center;
                justify-content: flex-start;
                margin-bottom: 10px;

                &__title {
                    font-family: 'font_medium', sans-serif;
                    font-size: 22px;
                    line-height: 27px;
                    color: #354049;
                    margin-right: 15px;
                }

                &__loading-image {
                    width: 18px;
                    height: 18px;
                }
            }

            &__blur {
                position: absolute;
                top: 0;
                left: 0;
                height: 100%;
                width: 100%;
                background-color: rgba(229, 229, 229, 0.2);
                z-index: 100;
            }
        }
    }

    .full-input {
        width: 100%;
        margin-top: 25px;
    }

    @media screen and (max-width: 1450px) {

        .new-project-step {
            padding: 0 150px;
        }
    }

    @media screen and (max-width: 900px) {

        .new-project-step {
            padding: 0 50px;
        }
    }
</style>

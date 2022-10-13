// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <img
                    class="modal__icon"
                    src="@/../static/images/account/billing/paidTier/prompt.png"
                    alt="Prompt Image"
                >
                <h1 class="modal__title" aria-roledescription="modal-title">
                    Create a Project
                </h1>
                <VInput
                    label="Project Name*"
                    additional-label="Up To 20 Characters"
                    placeholder="Project Name"
                    class="full-input"
                    is-limit-shown="true"
                    :current-limit="projectName.length"
                    :max-symbols="20"
                    :error="nameError"
                    @setData="setProjectName"
                />
                <VInput
                    label="Description - Optional"
                    placeholder="Project Description"
                    class="full-input"
                    is-multiline="true"
                    height="100px"
                    is-limit-shown="true"
                    :current-limit="description.length"
                    :max-symbols="100"
                    @setData="setProjectDescription"
                />
                <div class="modal__button-container">
                    <VButton
                        label="Cancel"
                        width="100%"
                        height="48px"
                        :on-press="closeModal"
                        is-transparent="true"
                    />
                    <VButton
                        label="Create Project"
                        width="100%"
                        height="48px"
                        :on-press="onCreateProjectClick"
                        :is-disabled="!projectName"
                    />
                </div>
                <div v-if="isLoading" class="modal__blur">
                    <VLoader class="modal__blur__loader" width="50px" height="50px" />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { RouteConfig } from '@/router';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { ProjectFields } from '@/types/projects';
import { LocalData } from '@/utils/localData';
import { AnalyticsHttpApi } from '@/api/analytics';

import VLoader from '@/components/common/VLoader.vue';
import VInput from '@/components/common/VInput.vue';
import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';

// @vue/component
@Component({
    components: {
        VButton,
        VModal,
        VInput,
        VLoader,
    },
})
export default class CreateProjectModal extends Vue {
    private description = '';
    private createdProjectId = '';
    private isLoading = false;

    public projectName = '';
    public nameError = '';

    public readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

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
        } catch (error) {
            this.isLoading = false;

            return;
        }

        this.selectCreatedProject();

        await this.$notify.success('Project created successfully!');

        this.isLoading = false;
        this.closeModal();

        this.analytics.pageVisit(RouteConfig.ProjectDashboard.path);
        await this.$router.push(RouteConfig.ProjectDashboard.path);
    }

    /**
     * Selects just created project.
     */
    private selectCreatedProject(): void {
        this.$store.dispatch(PROJECTS_ACTIONS.SELECT, this.createdProjectId);
        LocalData.setSelectedProjectId(this.createdProjectId);
    }

    /**
     * Holds on button click logic.
     * Closes this modal and opens create project modal.
     */
    public onClick(): void {
        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_CREATE_PROJECT_POPUP);
    }

    /**
     * Closes create project modal.
     */
    public closeModal(): void {
        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_CREATE_PROJECT_POPUP);
    }
}
</script>

<style scoped lang="scss">
    .modal {
        width: 400px;
        padding: 54px 48px 51px;
        display: flex;
        align-items: center;
        flex-direction: column;
        font-family: 'font_regular', sans-serif;

        @media screen and (max-width: 550px) {
            width: calc(100% - 48px);
            padding: 54px 24px 32px;
        }

        &__icon {
            max-height: 154px;
            max-width: 118px;

            @media screen and (max-width: 550px) {
                max-height: 77px;
                max-width: 59px;
            }
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            line-height: 34px;
            color: #1b2533;
            margin-top: 40px;
            text-align: center;

            @media screen and (max-width: 550px) {
                margin-top: 16px;
                font-size: 24px;
                line-height: 31px;
            }
        }

        &__info {
            font-family: 'font_regular', sans-serif;
            font-size: 16px;
            line-height: 21px;
            text-align: center;
            color: #354049;
            margin: 15px 0 45px;
        }

        &__button-container {
            width: 100%;
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-top: 30px;
            column-gap: 20px;

            @media screen and (max-width: 550px) {
                margin-top: 20px;
                column-gap: unset;
                row-gap: 8px;
                flex-direction: column-reverse;
            }
        }

        &__blur {
            position: absolute;
            top: 0;
            left: 0;
            height: 100%;
            width: 100%;
            background-color: rgb(229 229 229 / 20%);
            border-radius: 8px;
            z-index: 100;

            &__loader {
                width: 25px;
                height: 25px;
                position: absolute;
                right: 40px;
                top: 40px;
            }
        }
    }

    .full-input {
        margin-top: 20px;
    }

    @media screen and (max-width: 550px) {

        :deep(.add-label) {
            display: none;
        }
    }
</style>

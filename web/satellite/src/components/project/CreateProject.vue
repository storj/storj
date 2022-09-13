// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="create-project">
        <div class="create-project__container">
            <div class="create-project__container__image-container">
                <img
                    class="create-project__container__image-container__img"
                    src="@/../static/images/project/createProject.png"
                    alt="create project"
                >
            </div>
            <h2 class="create-project__container__title" aria-roledescription="title">Create a Project</h2>
            <VInput
                label="Project Name"
                additional-label="Up To 20 Characters"
                placeholder="Enter Project Name"
                class="full-input"
                is-limit-shown="true"
                :current-limit="projectName.length"
                :max-symbols="20"
                :error="nameError"
                @setData="setProjectName"
            />
            <VInput
                label="Description"
                placeholder="Enter Project Description"
                additional-label="Optional"
                class="full-input"
                is-multiline="true"
                height="100px"
                is-limit-shown="true"
                :current-limit="description.length"
                :max-symbols="100"
                @setData="setProjectDescription"
            />
            <div class="create-project__container__button-container">
                <VButton
                    class="create-project__container__button-container__cancel"
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
            <div v-if="isLoading" class="create-project__container__blur">
                <VLoader class="create-project__container__blur__loader" width="50px" height="50px" />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { ProjectFields } from '@/types/projects';
import { LocalData } from '@/utils/localData';
import { AnalyticsHttpApi } from '@/api/analytics';

import VLoader from '@/components/common/VLoader.vue';
import VButton from '@/components/common/VButton.vue';
import VInput from '@/components/common/VInput.vue';

// @vue/component
@Component({
    components: {
        VInput,
        VButton,
        VLoader,
    },
})
export default class NewProjectPopup extends Vue {
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
        } catch (error) {
            this.isLoading = false;

            return;
        }

        this.selectCreatedProject();

        await this.$notify.success('Project created successfully!');

        this.isLoading = false;

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
}
</script>

<style scoped lang="scss">
    .create-project {
        width: 100%;
        height: calc(100% - 140px);
        padding: 70px 0;
        font-family: 'font_regular', sans-serif;

        &__container {
            margin: 0 auto;
            max-width: 440px;
            padding: 70px 50px 55px;
            background-color: #fff;
            border-radius: 8px;
            position: relative;

            &__image-container {
                width: 100%;
                display: flex;
                justify-content: center;
            }

            &__img {
                max-width: 190px;
                max-height: 130px;
            }

            &__title {
                font-size: 28px;
                line-height: 34px;
                color: #384b65;
                font-family: 'font_bold', sans-serif;
                text-align: center;
                margin: 15px 0 30px;
            }

            &__button-container {
                width: 100%;
                display: flex;
                align-items: center;
                justify-content: space-between;
                margin-top: 30px;

                &__cancel {
                    margin-right: 20px;
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
    }

    .full-input {
        margin-top: 20px;
    }
</style>

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="isPopupShown" class="new-project-popup-container" @keyup.enter="createProjectClick" @keyup.esc="onCloseClick">
        <div class="new-project-popup" id="newProjectPopup" >
            <img class="new-project-popup__image" src="@/../static/images/project/createProject.jpg" alt="create project image">
            <div class="new-project-popup__form-container">
                <div class="new-project-popup__form-container__success-title-area">
                    <SuccessIcon/>
                    <p class="new-project-popup__form-container__success-title-area__title">Payment Method Added</p>
                </div>
                <h2 class="new-project-popup__form-container__main-title">Next, let’s create a project.</h2>
                <HeaderedInput
                    label="Project Name"
                    additional-label="Up To 20 Characters"
                    placeholder="Enter Project Name"
                    class="full-input"
                    width="100%"
                    max-symbols="20"
                    :error="nameError"
                    @setData="setProjectName"
                />
                <HeaderedInput
                    label="Description"
                    placeholder="Enter Project Description"
                    additional-label="Optional"
                    class="full-input"
                    is-multiline="true"
                    height="100px"
                    width="100%"
                    @setData="setProjectDescription"
                />
                <div class="new-project-popup__form-container__button-container">
                    <VButton
                        label="Back to Billing"
                        width="205px"
                        height="48px"
                        :on-press="onCloseClick"
                        is-white="true"
                    />
                    <VButton
                        label="Next"
                        width="205px"
                        height="48px"
                        :on-press="createProjectClick"
                    />
                </div>
            </div>
            <div class="new-project-popup__close-cross-container" @click="onCloseClick">
                <CloseCrossIcon/>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderedInput from '@/components/common/HeaderedInput.vue';
import VButton from '@/components/common/VButton.vue';

import CloseCrossIcon from '@/../static/images/common/closeCross.svg';
import SuccessIcon from '@/../static/images/project/success.svg';

import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { CreateProjectModel, Project } from '@/types/projects';
import {
    API_KEYS_ACTIONS,
    APP_STATE_ACTIONS,
    PM_ACTIONS,
} from '@/utils/constants/actionNames';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';

@Component({
    components: {
        HeaderedInput,
        VButton,
        CloseCrossIcon,
        SuccessIcon,
    },
})
export default class NewProjectPopup extends Vue {
    private projectName: string = '';
    private description: string = '';
    private createdProjectId: string = '';
    private isLoading: boolean = false;

    public nameError: string = '';

    /**
     * Indicates if popup is shown.
     */
    public get isPopupShown(): boolean {
        return this.$store.state.appStateModule.appState.isNewProjectPopupShown;
    }

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
     * Closes popup.
     */
    public onCloseClick(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_NEW_PROJ);
    }

    /**
     * Creates project and refreshes store.
     */
    public async createProjectClick(): Promise<void> {
        if (this.isLoading) {
            return;
        }

        this.isLoading = true;

        if (!this.validateProjectName()) {
            this.isLoading = false;

            return;
        }

        try {
            const project = await this.createProject();
            this.createdProjectId = project.id;
            this.$segment.track(SegmentEvent.PROJECT_CREATED, {
                project_id: this.createdProjectId,
            });
        } catch (error) {
            this.isLoading = false;
            await this.$notify.error(error.message);
            await this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_NEW_PROJ);

            return;
        }

        this.selectCreatedProject();

        try {
            await this.fetchProjectMembers();
        } catch (error) {
            await this.$notify.error(error.message);
        }

        try {
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_BILLING_HISTORY);
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_BALANCE);
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, this.createdProjectId);
        } catch (error) {
            await this.$notify.error(error.message);
        }

        this.clearApiKeys();

        this.clearBucketUsage();

        await this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_NEW_PROJ);

        this.checkIfUsersFirstProject();

        this.isLoading = false;
    }

    /**
     * Validates input value to satisfy project name rules.
     */
    private validateProjectName(): boolean {
        this.projectName = this.projectName.trim();

        const rgx = /^[^/]+$/;
        if (!rgx.test(this.projectName)) {
            this.nameError = 'Name for project is invalid!';

            return false;
        }

        if (this.projectName.length > 20) {
            this.nameError = 'Name should be less than 21 character!';

            return false;
        }

        return true;
    }

    /**
     * Makes create project request.
     */
    private async createProject(): Promise<Project> {
        const project: CreateProjectModel = {
            name: this.projectName,
            description: this.description,
            ownerId: this.$store.getters.user.id,
        };

        return await this.$store.dispatch(PROJECTS_ACTIONS.CREATE, project);
    }

    /**
     * Selects just created project.
     */
    private selectCreatedProject(): void {
        this.$store.dispatch(PROJECTS_ACTIONS.SELECT, this.createdProjectId);

        this.$store.dispatch(APP_STATE_ACTIONS.HIDE_CREATE_PROJECT_BUTTON);
    }

    /**
     * Indicates if user created his first project.
     */
    private checkIfUsersFirstProject(): void {
        const usersProjects: Project[] = this.$store.getters.projects.filter((project: Project) => project.ownerId === this.$store.getters.user.id);
        const isUsersFirstProject = usersProjects.length === 1;

        isUsersFirstProject
            ? this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_PROJECT_CREATION_POPUP)
            : this.$notify.success('Project created successfully!');
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
        this.$store.dispatch(BUCKET_ACTIONS.SET_SEARCH, '');
        this.$store.dispatch(BUCKET_ACTIONS.CLEAR);
    }
}
</script>

<style scoped lang="scss">
    .new-project-popup-container {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background-color: rgba(134, 134, 148, 0.4);
        z-index: 1121;
        display: flex;
        justify-content: center;
        align-items: center;
    }

    .input-container.full-input {
        width: 100%;
    }

    .new-project-popup {
        max-width: 970px;
        height: auto;
        background-color: #fff;
        border-radius: 6px;
        display: flex;
        flex-direction: row;
        align-items: center;
        position: relative;
        justify-content: center;
        padding: 70px 80px 70px 50px;

        &__image {
            min-height: 400px;
            min-width: 500px;
        }

        &__info-panel-container {
            display: flex;
            flex-direction: column;
            justify-content: flex-start;
            align-items: center;
            margin-right: 55px;
            height: 535px;

            &__main-label-text {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 39px;
                color: #384b65;
                margin-bottom: 60px;
                margin-top: 50px;
            }
        }

        &__form-container {
            width: 100%;
            max-width: 520px;

            &__success-title-area {
                display: flex;
                align-items: center;
                justify-content: flex-start;

                &__title {
                    font-size: 20px;
                    line-height: 24px;
                    color: #34bf89;
                    margin: 0 0 0 5px;
                }
            }

            &__main-title {
                font-size: 32px;
                line-height: 39px;
                color: #384b65;
            }

            &__button-container {
                width: 100%;
                display: flex;
                flex-direction: row;
                justify-content: space-between;
                align-items: center;
                margin-top: 30px;
            }
        }

        &__close-cross-container {
            display: flex;
            justify-content: center;
            align-items: center;
            position: absolute;
            right: 30px;
            top: 40px;
            height: 24px;
            width: 24px;
            cursor: pointer;

            &:hover .close-cross-svg-path {
                fill: #2683ff;
            }
        }
    }

    @media screen and (max-width: 720px) {

        .new-project-popup {

            &__info-panel-container {
                display: none;
            }

            &__form-container {

                &__button-container {
                    width: 100%;
                }
            }
        }
    }
</style>

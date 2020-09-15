// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="delete-project-popup-container">
        <div class="delete-project-popup" id="deleteProjectPopup">
            <div class="delete-project-popup__info-panel-container">
                <h2 class="delete-project-popup__info-panel-container__main-label-text">Delete Project</h2>
                <DeleteProjectIcon/>
            </div>
            <div class="delete-project-popup__form-container">
                <p class="delete-project-popup__form-container__confirmation-text">Are you sure that you want to delete your project? You will lose all your buckets and files that linked to this project.</p>
                <div>
                    <p class="text" v-if="!nameError">To confirm, enter the project name</p>
                    <div v-if="nameError" class="delete-project-popup__form-container__label">
                        <ErrorIcon alt="Red error icon with explanation mark"/>
                        <p class="text">{{nameError}}</p>
                    </div>
                    <input
                        class="delete-project-input"
                        type="text"
                        placeholder="Enter Project Name"
                        v-model="projectName"
                        @keyup="resetError"
                    />
                </div>
                <div class="delete-project-popup__form-container__button-container">
                    <VButton
                        label="Cancel"
                        width="205px"
                        height="48px"
                        :on-press="onCloseClick"
                        is-transparent="true"
                    />
                    <VButton
                        label="Delete"
                        width="205px"
                        height="48px"
                        class="red"
                        :on-press="onDeleteProjectClick"
                        :is-disabled="isDeleteButtonDisabled"
                    />
                </div>
            </div>
            <div class="delete-project-popup__close-cross-container" @click="onCloseClick">
                <CloseCrossIcon/>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import CloseCrossIcon from '@/../static/images/common/closeCross.svg';
import DeleteProjectIcon from '@/../static/images/project/deleteProject.svg';
import ErrorIcon from '@/../static/images/register/ErrorInfo.svg';

import { API_KEYS_ACTIONS } from '@/store/modules/apiKeys';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import {
    APP_STATE_ACTIONS,
    PM_ACTIONS,
} from '@/utils/constants/actionNames';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';

@Component({
    components: {
        VButton,
        DeleteProjectIcon,
        ErrorIcon,
        CloseCrossIcon,
    },
})
export default class DeleteProjectPopup extends Vue {
    private projectName: string = '';
    private nameError: string = '';
    private isLoading: boolean = false;

    public resetError (): void {
        this.nameError = '';
    }

    /**
     * If entered project name matches tries to delete project and select another.
     */
    public async onDeleteProjectClick(): Promise<void> {
        if (this.isLoading) {
            return;
        }

        if (!this.validateProjectName()) {
            return;
        }

        this.isLoading = true;

        try {
            await this.$store.dispatch(PROJECTS_ACTIONS.DELETE, this.$store.getters.selectedProject.id);
            this.$segment.track(SegmentEvent.PROJECT_DELETED, {
                project_id: this.$store.getters.selectedProject.id,
            });
            await this.$notify.success('Project was successfully deleted');

            await this.selectProject();
        } catch (e) {
            await this.$notify.error(e.message);
        }

        this.isLoading = false;

        await this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_DEL_PROJ);
    }

    /**
     * Closes popup.
     */
    public onCloseClick(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_DEL_PROJ);
    }

    /**
     * Indicates if delete button is disabled when project name is not entered or incorrect.
     */
    public get isDeleteButtonDisabled(): boolean {
        return !this.projectName || !!this.nameError;
    }

    /**
     * Checks is entered project name matches selected.
     */
    private validateProjectName(): boolean {
        if (this.projectName === this.$store.getters.selectedProject.name) {
            return true;
        }

        this.nameError = 'Name doesn\'t match with current project name';
        this.isLoading = false;

        return false;
    }

    private async selectProject(): Promise<void> {
        if (this.$store.state.projectsModule.projects.length === 0) {
            await this.$store.dispatch(PM_ACTIONS.CLEAR);
            await this.$store.dispatch(API_KEYS_ACTIONS.CLEAR);
            await this.$store.dispatch(BUCKET_ACTIONS.CLEAR);

            return;
        }

        // TODO: reuse select project functionality
        await this.$store.dispatch(PROJECTS_ACTIONS.SELECT, this.$store.state.projectsModule.projects[0].id);
        await this.$store.dispatch(PM_ACTIONS.FETCH, 1);
        await this.$store.dispatch(API_KEYS_ACTIONS.FETCH, 1);
        await this.$store.dispatch(BUCKET_ACTIONS.FETCH, 1);
    }
}
</script>

<style scoped lang="scss">
    .delete-project-popup-container {
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
        font-family: 'font_medium', sans-serif;
    }

    .input-container.full-input {
        width: 100%;
    }

    .red {
        background-color: #eb5757;
    }

    .delete-project-popup {
        width: 100%;
        max-width: 800px;
        height: 460px;
        background-color: #fff;
        border-radius: 6px;
        display: flex;
        flex-direction: row;
        align-items: center;
        position: relative;
        justify-content: space-between;
        padding: 20px 100px 0 100px;

        &__info-panel-container {
            display: flex;
            flex-direction: column;
            justify-content: flex-start;
            align-items: center;
            margin-right: 55px;

            &__main-label-text {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 39px;
                color: #384b65;
                margin-bottom: 30px;
                margin-top: 0;
            }
        }

        &__form-container {
            width: 100%;
            max-width: 440px;
            height: 335px;

            &__confirmation-text {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 21px;
                margin-bottom: 30px;
            }

            &__label {
                display: flex;
                flex-direction: row;
                align-items: center;

                .text {
                    font-family: 'font_medium', sans-serif;
                    padding-left: 10px;
                    color: #eb5757;
                }
            }

            .text {
                margin: 0;
            }

            .delete-project-input {
                font-family: 'font_regular', sans-serif;
                font-size: 16px;
                line-height: 21px;
                margin-top: 10px;
                resize: none;
                margin-bottom: 18px;
                height: 48px;
                width: 100%;
                text-indent: 20px;
                border-color: rgba(56, 75, 101, 0.4);
                border-radius: 6px;
                outline: none;
                box-shadow: none;
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

        .delete-project-popup {

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

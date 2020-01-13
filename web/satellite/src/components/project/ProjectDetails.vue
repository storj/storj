// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="project-details">
            <h1 class="project-details__title">Project Details</h1>
            <div class="project-details-info-container">
                <div class="project-details-info-container__name-container">
                    <h2 class="project-details-info-container__name-container__title">Project Name</h2>
                    <h3 class="project-details-info-container__name-container__project-name">{{name}}</h3>
                </div>
            </div>
            <div class="project-details-info-container">
                <div class="project-details-info-container__description-container" v-if="!isEditing">
                    <div class="project-details-info-container__description-container__text">
                        <h2 class="project-details-info-container__description-container__text__title">Description</h2>
                        <h3 class="project-details-info-container__description-container__text__project-description">{{displayedDescription}}</h3>
                    </div>
                    <EditIcon
                        class="project-details-svg"
                        @click="toggleEditing"
                    />
                </div>
                <div class="project-details-info-container__description-container--editing" v-if="isEditing">
                    <HeaderedInput
                        label="Description"
                        placeholder="Enter Description"
                        width="205%"
                        height="10vh"
                        is-multiline="true"
                        :init-value="storedDescription"
                        @setData="setNewDescription"
                    />
                    <div class="project-details-info-container__description-container__buttons-area">
                        <VButton
                            label="Cancel"
                            width="180px"
                            height="48px"
                            :on-press="toggleEditing"
                            is-white="true"
                        />
                        <VButton
                            label="Save"
                            width="180px"
                            height="48px"
                            :on-press="onSaveButtonClick"
                        />
                    </div>
                </div>
            </div>
            <div class="project-details-info-container">
                <ProjectLimitsArea />
            </div>
            <p class="project-details__limits-increase-text">
                To increase your limits please contact us at
                <a
                    href="mailto:support@tardigrade.io"
                    class="project-details__limits-increase-text__link"
                >
                    support@tardigrade.io
                </a>
            </p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import EmptyState from '@/components/common/EmptyStateArea.vue';
import HeaderedInput from '@/components/common/HeaderedInput.vue';
import VButton from '@/components/common/VButton.vue';
import DeleteProjectPopup from '@/components/project/DeleteProjectPopup.vue';
import ProjectLimitsArea from '@/components/project/ProjectLimitsArea.vue';

import EditIcon from '@/../static/images/project/edit.svg';

import { RouteConfig } from '@/router';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { UpdateProjectModel } from '@/types/projects';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';

@Component({
    components: {
        VButton,
        HeaderedInput,
        EmptyState,
        DeleteProjectPopup,
        EditIcon,
        ProjectLimitsArea,
    },
})
export default class ProjectDetailsArea extends Vue {
    private isEditing: boolean = false;
    private newDescription: string = '';

    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch(PROJECTS_ACTIONS.FETCH);
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, this.$store.getters.selectedProject.id);
            this.$segment.track(SegmentEvent.PROJECT_VIEWED, {
                project_id: this.$store.getters.selectedProject.id,
            });
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    public get name(): string {
        return this.$store.getters.selectedProject.name;
    }

    public get storedDescription(): string {
        return this.$store.getters.selectedProject.description;
    }

    public get displayedDescription(): string {
        return this.$store.getters.selectedProject.description ?
            this.$store.getters.selectedProject.description :
            'No description yet. Please enter some information about the project if any.';
    }

    public get isPopupShown(): boolean {
        return this.$store.state.appStateModule.appState.isDeleteProjectPopupShown;
    }

    public setNewDescription(value: string): void {
        this.newDescription = value;
    }

    public async onSaveButtonClick(): Promise<void> {
        try {
            await this.$store.dispatch(
                PROJECTS_ACTIONS.UPDATE,
                new UpdateProjectModel(this.$store.getters.selectedProject.id, this.newDescription),
            );
        } catch (error) {
            await this.$notify.error(`Unable to update project description. ${error.message}`);

            return;
        }

        this.toggleEditing();
        await this.$notify.success('Project updated successfully!');
    }

    public toggleDeleteDialog(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_DEL_PROJ);
    }

    public onMoreClick(): void {
        this.$router.push(RouteConfig.UsageReport.path);
    }

    public toggleEditing(): void {
        this.isEditing = !this.isEditing;
        this.newDescription = this.storedDescription;
    }
}
</script>

<style scoped lang="scss">
    h1,
    h2,
    h3 {
        margin-block-start: 0.5em;
        margin-block-end: 0.5em;
    }

    .project-details {
        position: relative;
        overflow: hidden;
        font-family: 'font_regular', sans-serif;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            line-height: 39px;
            color: #263549;
            user-select: none;
            margin: 0;
        }

        &__button-area {
            margin-top: 3vh;
            margin-bottom: 100px;
        }

        &__limits-increase-text {
            font-family: 'font_regular', sans-serif;
            font-size: 16px;
            color: #afb7c1;
            margin-top: 42px;

            &__link {
                text-decoration: underline;
                color: #2683ff;
            }
        }
    }

    .project-details-info-container {
        height: auto;
        margin-top: 37px;
        display: flex;
        flex-direction: row;
        justify-content: space-between;
        align-items: flex-start;

        &__name-container {
            min-height: 67px;
            width: 100%;
            border-radius: 6px;
            display: flex;
            flex-direction: column;
            justify-content: center;
            align-items: flex-start;
            padding: 28px;
            background-color: #fff;

            &__title {
                font-size: 16px;
                line-height: 21px;
                color: rgba(56, 75, 101, 0.4);
                user-select: none;
            }

            &__project-name {
                font-size: 16px;
                line-height: 21px;
                color: #354049;
            }
        }

        &__description-container {
            min-height: 67px;
            width: 100%;
            border-radius: 6px;
            display: flex;
            height: auto;
            flex-direction: row;
            justify-content: space-between;
            align-items: center;
            padding: 28px;
            background-color: #fff;

            ::-webkit-scrollbar,
            ::-webkit-scrollbar-track,
            ::-webkit-scrollbar-thumb {
                margin-top: 0;
            }

            &__text {
                display: flex;
                flex-direction: column;
                justify-content: center;
                align-items: flex-start;
                margin-right: 20px;
                width: 100%;

                &__title {
                    font-size: 16px;
                    line-height: 21px;
                    color: rgba(56, 75, 101, 0.4);
                    user-select: none;
                }

                &__project-description {
                    font-size: 16px;
                    line-height: 21px;
                    color: #354049;
                    width: 100%;
                    max-height: 25vh;
                    overflow-y: scroll;
                    word-break: break-word;
                    white-space: pre-line;
                }
            }

            &--editing {
                min-height: 67px;
                width: 100%;
                border-radius: 6px;
                display: flex;
                height: auto;
                padding: 28px;
                background-color: #fff;
                flex-direction: column;
                justify-content: center;
                align-items: flex-start;
            }

            &__buttons-area {
                margin-top: 2vh;
                display: flex;
                flex-direction: row;
                align-items: center;
                width: 380px;
                justify-content: space-between;
            }

            .project-details-svg {
                cursor: pointer;
                min-width: 40px;

                &:hover {

                    .project-details-svg__rect {
                        fill: #2683ff;
                    }

                    .project-details-svg__path {
                        fill: white;
                    }
                }
            }
        }

        &__portability-container {
            min-height: 67px;
            width: 100%;
            border-radius: 6px;
            display: flex;
            height: auto;
            flex-direction: row;
            justify-content: space-between;
            align-items: center;
            padding: 28px;
            background-color: #fff;

            &__info {
                display: flex;
                flex-direction: row;
                align-items: center;

                &__text {
                    margin-left: 2vw;
                }
            }

            &__buttons-area {
                display: flex;
                flex-direction: row;
                align-items: center;
                width: 380px;
                justify-content: space-between;
            }
        }
    }
</style>

// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-details">
        <h1 class="project-details__title">Project Details</h1>
        <div class="project-details__name-container">
            <p class="project-details__name-container__title">Project Name</p>
            <p class="project-details__name-container__project-name">{{name}}</p>
        </div>
        <div class="project-details__description-container" v-if="!isEditing">
            <div class="project-details__description-container__text-area">
                <p class="project-details__description-container__text-area__title">Description</p>
                <p class="project-details__description-container__text-area__project-description">{{displayedDescription}}</p>
            </div>
            <EditIcon
                class="project-details-svg"
                @click="toggleEditing"
            />
        </div>
        <div class="project-details__description-container__editing" v-if="isEditing">
            <HeaderedInput
                label="Description"
                placeholder="Enter Description"
                width="205%"
                height="10vh"
                is-multiline="true"
                :init-value="storedDescription"
                @setData="setNewDescription"
            />
            <div class="project-details__description-container__editing__buttons-area">
                <VButton
                    label="Cancel"
                    width="180px"
                    height="48px"
                    :on-press="toggleEditing"
                    is-white="true"
                />
                <VButton
                    class="save-button"
                    label="Save"
                    width="180px"
                    height="48px"
                    :on-press="onSaveButtonClick"
                />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderedInput from '@/components/common/HeaderedInput.vue';
import VButton from '@/components/common/VButton.vue';

import EditIcon from '@/../static/images/project/edit.svg';

import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { UpdateProjectModel } from '@/types/projects';

@Component({
    components: {
        VButton,
        HeaderedInput,
        EditIcon,
    },
})
export default class ProjectDetails extends Vue {
    public isEditing: boolean = false;
    private newDescription: string = '';

    /**
     * Returns selected project name.
     */
    public get name(): string {
        return this.$store.getters.selectedProject.name;
    }

    /**
     * Returns selected project description from store.
     */
    public get storedDescription(): string {
        return this.$store.getters.selectedProject.description;
    }

    /**
     * Returns displayed project description on UI.
     */
    public get displayedDescription(): string {
        return this.storedDescription ?
            this.storedDescription :
            'No description yet. Please enter some information about the project if any.';
    }

    /**
     * Sets new description from value string.
     */
    public setNewDescription(value: string): void {
        this.newDescription = value;
    }

    /**
     * Updates project description.
     */
    public async onSaveButtonClick(): Promise<void> {
        try {
            const updatedProject = new UpdateProjectModel(this.$store.getters.selectedProject.id, this.newDescription);
            await this.$store.dispatch(PROJECTS_ACTIONS.UPDATE, updatedProject);
        } catch (error) {
            await this.$notify.error(`Unable to update project description. ${error.message}`);

            return;
        }

        this.toggleEditing();
        await this.$notify.success('Project updated successfully!');
    }

    /**
     * Toggles project description editing state.
     */
    public toggleEditing(): void {
        this.isEditing = !this.isEditing;
        this.newDescription = this.storedDescription;
    }
}
</script>

<style scoped lang="scss">
    h1,
    p {
        margin: 0;
    }

    .project-details {
        padding: 30px;
        margin-right: 32px;
        width: calc(30% - 60px);
        font-family: 'font_regular', sans-serif;
        background-color: #fff;
        border-radius: 6px;

        &__title {
            font-family: 'font_medium', sans-serif;
            font-size: 18px;
            line-height: 18px;
            color: #354049;
            margin-bottom: 25px;
        }

        &__name-container {
            width: 100%;
            margin-bottom: 35px;

            &__title {
                font-size: 16px;
                line-height: 16px;
                color: rgba(56, 75, 101, 0.4);
                margin-bottom: 15px;
            }

            &__project-name {
                font-size: 16px;
                line-height: 16px;
                color: #354049;
            }
        }

        &__description-container {
            display: flex;
            align-items: center;
            justify-content: space-between;
            width: 100%;

            &__text-area {
                width: 100%;

                &__title {
                    font-size: 16px;
                    line-height: 16px;
                    color: rgba(56, 75, 101, 0.4);
                    margin-bottom: 15px;
                }

                &__project-description {
                    font-size: 16px;
                    line-height: 24px;
                    color: #354049;
                    max-height: 48px;
                    width: available;
                    overflow-y: scroll;
                    word-break: break-word;
                    white-space: pre-line;
                }
            }

            &__editing {

                &__buttons-area {
                    display: flex;
                    align-items: center;
                    justify-content: flex-end;
                    margin-top: 20px;

                    .save-button {
                        margin-left: 10px;
                    }
                }
            }
        }
    }

    .project-details-svg {
        margin-left: 20px;
        min-width: 40px;
        cursor: pointer;

        &:hover {

            .project-details-svg__rect {
                fill: #2683ff;
            }

            .project-details-svg__path {
                fill: white;
            }
        }
    }
</style>

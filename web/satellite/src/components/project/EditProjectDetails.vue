// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-details">
        <div class="project-details__wrapper">
            <p class="project-details__wrapper__back" @click.stop.prevent="onBackClick"><- Back</p>
            <div class="project-details__wrapper__container">
                <h1 class="project-details__wrapper__container__title">Project Details</h1>
                <p class="project-details__wrapper__container__label">Name</p>
                <div class="project-details__wrapper__container__name-area" v-if="!isNameEditing">
                    <p class="project-details__wrapper__container__name-area__name">{{ storedProject.name }}</p>
                    <VButton
                        label="Edit"
                        width="64px"
                        height="28px"
                        :on-press="toggleNameEditing"
                        is-white="true"
                    />
                </div>
                <div class="project-details__wrapper__container__name-editing" v-if="isNameEditing">
                    <input
                        class="project-details__wrapper__container__name-editing__input"
                        placeholder="Enter a name for your project"
                        @input="onNameInput"
                        @change="onNameInput"
                        v-model="nameValue"
                    />
                    <span class="project-details__wrapper__container__name-editing__limit">{{ nameValue.length }}/{{ nameLength }}</span>
                    <VButton
                        class="project-details__wrapper__container__name-editing__save-button"
                        label="Save"
                        width="66px"
                        height="30px"
                        :on-press="onSaveNameButtonClick"
                    />
                </div>
                <p class="project-details__wrapper__container__label">Description</p>
                <div class="project-details__wrapper__container__description-area" v-if="!isDescriptionEditing">
                    <p class="project-details__wrapper__container__description-area__description">{{ displayedDescription }}</p>
                    <VButton
                        label="Edit"
                        width="64px"
                        height="28px"
                        :on-press="toggleDescriptionEditing"
                        is-white="true"
                    />
                </div>
                <div class="project-details__wrapper__container__description-editing" v-if="isDescriptionEditing">
                    <input
                        class="project-details__wrapper__container__description-editing__input"
                        placeholder="Enter a description for your project"
                        @input="onDescriptionInput"
                        @change="onDescriptionInput"
                        v-model="descriptionValue"
                    />
                    <span class="project-details__wrapper__container__description-editing__limit">{{ descriptionValue.length }}/{{ descriptionLength }}</span>
                    <VButton
                        class="project-details__wrapper__container__description-editing__save-button"
                        label="Save"
                        width="66px"
                        height="30px"
                        :on-press="onSaveDescriptionButtonClick"
                    />
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderedInput from '@/components/common/HeaderedInput.vue';
import VButton from '@/components/common/VButton.vue';

import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { MAX_DESCRIPTION_LENGTH, MAX_NAME_LENGTH, Project, ProjectFields } from '@/types/projects';

@Component({
    components: {
        VButton,
        HeaderedInput,
    },
})
export default class EditProjectDetails extends Vue {
    public isNameEditing: boolean = false;
    public isDescriptionEditing: boolean = false;
    public nameValue: string = '';
    public descriptionValue: string = '';
    public nameLength: number = MAX_NAME_LENGTH;
    public descriptionLength: number = MAX_DESCRIPTION_LENGTH;

    /**
     * Returns selected project from store.
     */
    public get storedProject(): Project {
        return this.$store.getters.selectedProject;
    }

    /**
     * Returns displayed project description on UI.
     */
    public get displayedDescription(): string {
        return this.storedProject.description ?
            this.storedProject.description :
            'No description yet. Please enter some information if any.';
    }

    /**
     * Triggers on name input.
     */
    public onNameInput({ target }): void {
        if (target.value.length < MAX_NAME_LENGTH) {
            this.nameValue = target.value;

            return;
        }

        this.nameValue = target.value.slice(0, MAX_NAME_LENGTH);
    }

    /**
     * Triggers on description input.
     */
    public onDescriptionInput({ target }): void {
        if (target.value.length < MAX_DESCRIPTION_LENGTH) {
            this.descriptionValue = target.value;

            return;
        }

        this.descriptionValue = target.value.slice(0, MAX_DESCRIPTION_LENGTH);
    }

    /**
     * Updates project name.
     */
    public async onSaveNameButtonClick(): Promise<void> {
        try {
            const updatedProject = new ProjectFields(this.nameValue, '');
            updatedProject.checkName();

            await this.$store.dispatch(PROJECTS_ACTIONS.UPDATE_NAME, updatedProject);
        } catch (error) {
            await this.$notify.error(`Unable to update project name. ${error.message}`);

            return;
        }

        this.toggleNameEditing();
        await this.$notify.success('Project name updated successfully!');
    }

    /**
     * Updates project description.
     */
    public async onSaveDescriptionButtonClick(): Promise<void> {
        try {
            const updatedProject = new ProjectFields('', this.descriptionValue);
            await this.$store.dispatch(PROJECTS_ACTIONS.UPDATE_DESCRIPTION, updatedProject);
        } catch (error) {
            await this.$notify.error(`Unable to update project description. ${error.message}`);

            return;
        }

        this.toggleDescriptionEditing();
        await this.$notify.success('Project description updated successfully!');
    }

    /**
     * Toggles project name editing state.
     */
    public toggleNameEditing(): void {
        this.isNameEditing = !this.isNameEditing;
        this.nameValue = this.storedProject.name;
    }

    /**
     * Toggles project description editing state.
     */
    public toggleDescriptionEditing(): void {
        this.isDescriptionEditing = !this.isDescriptionEditing;
        this.descriptionValue = this.storedProject.description;
    }

    /**
     * Redirects to previous route.
     */
    public onBackClick(): void {
        const PREVIOUS_ROUTE_NUMBER = -1;

        this.$router.go(PREVIOUS_ROUTE_NUMBER);
    }
}
</script>

<style scoped lang="scss">
    .project-details {
        padding: 45px 0;
        font-family: 'font_regular', sans-serif;
        display: flex;
        align-items: center;
        justify-content: center;

        &__wrapper {
            width: 727px;

            &__back {
                width: fit-content;
                cursor: pointer;
                font-weight: 500;
                font-size: 16px;
                line-height: 23px;
                color: #2582ff;
                margin: 0 0 20px 0;
            }

            &__container {
                padding: 50px;
                width: calc(100% - 100px);
                border-radius: 6px;
                background-color: #fff;

                &__title {
                    font-family: 'font_bold', sans-serif;
                    font-size: 22px;
                    line-height: 27px;
                    color: #384b65;
                    margin: 0 0 35px 0;
                }

                &__label {
                    font-family: 'font_bold', sans-serif;
                    font-size: 16px;
                    line-height: 16px;
                    color: #384b65;
                    margin: 0 0 15px 0;
                }

                &__name-area,
                &__description-area {
                    display: flex;
                    align-items: center;
                    justify-content: space-between;
                    padding: 11px 7px 11px 23px;
                    width: calc(100% - 30px);
                    background: #f5f6fa;
                    border-radius: 6px;

                    &__name,
                    &__description {
                        font-weight: normal;
                        font-size: 16px;
                        line-height: 19px;
                        color: #384b65;
                        margin: 0;
                        word-break: break-all;
                    }

                    &__name {
                        font-weight: bold;
                    }
                }

                &__name-area {
                    margin-bottom: 35px;
                }

                &__name-editing,
                &__description-editing {
                    display: flex;
                    align-items: center;
                    width: calc(100% - 7px);
                    border-radius: 6px;
                    background-color: #f5f6fa;
                    padding-right: 7px;

                    &__input {
                        font-weight: normal;
                        font-size: 16px;
                        line-height: 21px;
                        flex: 1;
                        height: 48px;
                        width: available;
                        text-indent: 20px;
                        background-color: #f5f6fa;
                        border-color: #f5f6fa;
                        border-radius: 6px;

                        &::placeholder {
                            opacity: 0.6;
                        }
                    }

                    &__limit {
                        font-size: 14px;
                        line-height: 21px;
                        color: rgba(0, 0, 0, 0.3);
                        margin: 0 0 0 15px;
                        min-width: 53px;
                        text-align: right;
                    }

                    &__save-button {
                        margin-left: 15px;
                    }
                }

                &__name-editing {
                    margin-bottom: 35px;
                }
            }
        }
    }
</style>

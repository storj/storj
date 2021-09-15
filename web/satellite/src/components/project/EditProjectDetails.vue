// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-details">
        <div class="project-details__wrapper">
            <p class="project-details__wrapper__back" @click.stop.prevent="onBackClick">&lt;- Back</p>
            <div class="project-details__wrapper__container">
                <h1 class="project-details__wrapper__container__title">Project Details</h1>
                <p class="project-details__wrapper__container__label">Name</p>
                <div v-if="!isNameEditing" class="project-details__wrapper__container__name-area">
                    <p class="project-details__wrapper__container__name-area__name">{{ storedProject.name }}</p>
                    <VButton
                        label="Edit"
                        width="64px"
                        height="28px"
                        :on-press="toggleNameEditing"
                        is-white="true"
                    />
                </div>
                <div v-if="isNameEditing" class="project-details__wrapper__container__name-editing">
                    <input
                        v-model="nameValue"
                        class="project-details__wrapper__container__name-editing__input"
                        placeholder="Enter a name for your project"
                        @input="onNameInput"
                        @change="onNameInput"
                    >
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
                <div v-if="!isDescriptionEditing" class="project-details__wrapper__container__description-area">
                    <p class="project-details__wrapper__container__description-area__description">{{ displayedDescription }}</p>
                    <VButton
                        label="Edit"
                        width="64px"
                        height="28px"
                        :on-press="toggleDescriptionEditing"
                        is-white="true"
                    />
                </div>
                <div v-if="isDescriptionEditing" class="project-details__wrapper__container__description-editing">
                    <input
                        v-model="descriptionValue"
                        class="project-details__wrapper__container__description-editing__input"
                        placeholder="Enter a description for your project"
                        @input="onDescriptionInput"
                        @change="onDescriptionInput"
                    >
                    <span class="project-details__wrapper__container__description-editing__limit">{{ descriptionValue.length }}/{{ descriptionLength }}</span>
                    <VButton
                        class="project-details__wrapper__container__description-editing__save-button"
                        label="Save"
                        width="66px"
                        height="30px"
                        :on-press="onSaveDescriptionButtonClick"
                    />
                </div>
                <div v-if="isPaidTier" class="project-details__wrapper__container__limits">
                    <p class="project-details__wrapper__container__limits__label">Storage Limit</p>
                    <div v-if="!isStorageLimitEditing" class="project-details__wrapper__container__limits__storagelimit-area">
                        <p class="project-details__wrapper__container__limits__storagelimit-area__storagelimit">{{ storageLimitFormatted }}</p>
                        <VButton
                            label="Edit"
                            width="64px"
                            height="28px"
                            :on-press="toggleStorageLimitEditing"
                            is-white="true"
                        />
                    </div>
                    <div v-if="isStorageLimitEditing" class="project-details__wrapper__container__limits__storagelimit-editing">
                        <input
                            v-model="storageLimitValue"
                            class="project-details__wrapper__container__limits__storagelimit-editing__input"
                            placeholder="Enter a storage limit for your project"
                            @input="onStorageLimitInput"
                            @change="onStorageLimitInput"
                        >
                        <span class="project-details__wrapper__container__limits__storagelimit-editing__limit">{{ nameValue.length }}/{{ nameLength }}</span>
                        <VButton
                            class="project-details__wrapper__container__limits__storagelimit-editing__save-button"
                            label="Save"
                            width="66px"
                            height="30px"
                            :on-press="onSaveStorageLimitButtonClick"
                        />
                    </div>
                    <p class="project-details__wrapper__container__limits__label">Bandwidth Limit</p>
                    <div v-if="!isBandwidthLimitEditing" class="project-details__wrapper__container__limits__bandwidthlimit-area">
                        <p class="project-details__wrapper__container__limits__bandwidthlimit-area__bandwidthlimit">{{ bandwidthLimitFormatted }}</p>
                        <VButton
                            label="Edit"
                            width="64px"
                            height="28px"
                            :on-press="toggleBandwidthLimitEditing"
                            is-white="true"
                        />
                    </div>
                    <div v-if="isBandwidthLimitEditing" class="project-details__wrapper__container__limits__bandwidthlimit-editing">
                        <input
                            v-model="bandwidthLimitValue"
                            class="project-details__wrapper__container__limits__bandwidthlimit-editing__input"
                            placeholder="Enter a bandwidth limit for your project"
                            @input="onBandwidthLimitInput"
                            @change="onBandwidthLimitInput"
                        >
                        <span class="project-details__wrapper__container__limits__bandwidthlimit-editing__limit">{{ nameValue.length }}/{{ nameLength }}</span>
                        <VButton
                            class="project-details__wrapper__container__limits__bandwidthlimit-editing__save-button"
                            label="Save"
                            width="66px"
                            height="30px"
                            :on-press="onSaveBandwidthLimitButtonClick"
                        />
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Dimensions, Size } from '@/utils/bytesSize';
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import {
    MAX_DESCRIPTION_LENGTH,
    MAX_NAME_LENGTH,
    Project,
    ProjectFields, ProjectLimits
} from '@/types/projects';

// @vue/component
@Component({
    components: {
        VButton,
    },
})
export default class EditProjectDetails extends Vue {
    public isNameEditing = false;
    public isDescriptionEditing = false;
    public isStorageLimitEditing = false;
    public isBandwidthLimitEditing = false;
    public isPaidTier = false;
    public nameValue = '';
    public descriptionValue = '';
    public storageLimitValue = 0;
    public bandwidthLimitValue = 0;
    public nameLength: number = MAX_NAME_LENGTH;
    public descriptionLength: number = MAX_DESCRIPTION_LENGTH;

    /**
     * Returns selected project from store.
     */
    public get storedProject(): Project {
        return this.$store.getters.selectedProject;
    }

    /**
     * Lifecycle hook after initial render.
     * Fetches project limits and paid tier status.
     */
    public async mounted(): Promise<void> {
        if (!this.$store.getters.selectedProject.id) {
            return;
        }

        if (this.$store.state.usersModule.user.paidTier)
        {
            this.isPaidTier = true;
        }

        try {
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, this.$store.getters.selectedProject.id);
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * Returns current limits from store.
     */
    public get currentLimits(): ProjectLimits {
        return this.$store.state.projectsModule.currentLimits;
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
    public onNameInput(event: Event): void {
        const target = event.target as HTMLInputElement;
        if (target.value.length < MAX_NAME_LENGTH) {
            this.nameValue = target.value;
            return;
        }

        this.nameValue = target.value.slice(0, MAX_NAME_LENGTH);
    }

    /**
     * Triggers on description input.
     */
    public onDescriptionInput(event: Event): void {
        const target = event.target as HTMLInputElement;
        if (target.value.length < MAX_DESCRIPTION_LENGTH) {
            this.descriptionValue = target.value;
            return;
        }

        this.descriptionValue = target.value.slice(0, MAX_DESCRIPTION_LENGTH);
    }

    /**
     * Returns formatted limit amount.
     */
    public get storageLimitFormatted(): string {
        return this.formattedValue(new Size(this.currentLimits.storageLimit, 2));
    }

    /**
     * Triggers on storage limit input.
     */
    public onStorageLimitInput(event: Event): void {
        const target = event.target as HTMLInputElement;
        this.storageLimitValue = parseInt(target.value);
    }

    /**
     * Returns formatted limit amount.
     */
    public get bandwidthLimitFormatted(): string {
        return this.formattedValue(new Size(this.currentLimits.bandwidthLimit, 2));
    }

    /**
     * Triggers on bandwidth limit input.
     */
    public onBandwidthLimitInput(event: Event): void {
        const target = event.target as HTMLInputElement;
        this.bandwidthLimitValue = parseInt(target.value);
    }

    /**
     * Formats value to needed form and returns it.
     */
    private formattedValue(value: Size): string {
        switch (value.label) {
        case Dimensions.Bytes:
        case Dimensions.KB:
            return '0';
        default:
            return `${value.formattedBytes.replace(/\\.0+$/, '')}${value.label}`;
        }
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
     * Updates project storage limit.
     */
    public async onSaveStorageLimitButtonClick(): Promise<void> {
        try {
            const updatedProject = new ProjectLimits(0, 0, this.storageLimitValue);
            await this.$store.dispatch(PROJECTS_ACTIONS.UPDATE_STORAGE_LIMIT, updatedProject);
        } catch (error) {
            await this.$notify.error(`Unable to update project storage limit. ${error.message}`);

            return;
        }

        this.toggleStorageLimitEditing();
        await this.$notify.success('Project storage limit updated successfully!');
    }

    /**
     * Updates project bandwidth limit.
     */
    public async onSaveBandwidthLimitButtonClick(): Promise<void> {
        try {
            const updatedProject = new ProjectLimits(this.bandwidthLimitValue);
            await this.$store.dispatch(PROJECTS_ACTIONS.UPDATE_BANDWIDTH_LIMIT, updatedProject);
        } catch (error) {
            await this.$notify.error(`Unable to update project bandwidth limit. ${error.message}`);

            return;
        }

        this.toggleBandwidthLimitEditing();
        await this.$notify.success('Project bandwidth limit updated successfully!');
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
     * Toggles project storage limit editing state.
     */
    public toggleStorageLimitEditing(): void {
        if (this.$store.state.usersModule.user.paidTier) {
            this.isStorageLimitEditing = !this.isStorageLimitEditing;
            this.storageLimitValue = this.currentLimits.storageLimit;
        }
    }

    /**
     * Toggles project bandwidth limit editing state.
     */
    public toggleBandwidthLimitEditing(): void {
        if (this.$store.state.usersModule.user.paidTier) {
            this.isBandwidthLimitEditing = !this.isBandwidthLimitEditing;
            this.bandwidthLimitValue = this.currentLimits.bandwidthLimit;
        }
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
                &__description-area,
                &__limits__storagelimit-area,
                &__limits__bandwidthlimit-area {
                    display: flex;
                    align-items: center;
                    justify-content: space-between;
                    padding: 11px 7px 11px 23px;
                    width: calc(100% - 30px);
                    background: #f5f6fa;
                    border-radius: 6px;

                    &__name,
                    &__description,
                    &__limits__storagelimit,
                    &__limits__bandwidthlimit {
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

                &__name-area,
                &__description-area,
                &__limits__storagelimit-area {
                    margin-bottom: 35px;
                }

                &__name-editing,
                &__description-editing,
                &__limits__storagelimit-editing,
                &__limits__bandwidthlimit-editing {
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

                &__name-editing,
                &__description-editing,
                &___limits_storagelimit-editing {
                    margin-bottom: 35px;
                }
            }
        }
    }
</style>

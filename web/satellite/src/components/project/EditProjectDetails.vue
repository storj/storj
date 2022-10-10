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
                    <p class="project-details__wrapper__container__label">Storage Limit</p>
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
                    <div v-if="isStorageLimitEditing" class="project-details__wrapper__container__limits__storagelimit-editing__section">
                        <div class="project-details__wrapper__container__limits__storagelimit-editing__slider__wrapper">
                            <input
                                v-model="storageLimitValue"
                                class="project-details__wrapper__container__limits__storagelimit-editing__slider__input"
                                min="0"
                                :max="paidStorageLimit"
                                type="range"
                                @input="onStorageLimitInput"
                                @change="onStorageLimitInput"
                            >
                            <div class="project-details__wrapper__container__limits__storagelimit-editing__slider__label-wrapper">
                                <p class="project-details__wrapper__container__limits__storagelimit-editing__slider__label">0 {{ storageMeasurementFormatted }}</p>
                                <p class="project-details__wrapper__container__limits__storagelimit-editing__slider__label">{{ paidStorageLimit }}</p>
                            </div>
                        </div>
                        <div class="project-details__wrapper__container__limits__storagelimit-editing__units-wrapper">
                            <p
                                class="project-details__wrapper__container__limits__storagelimit-editing__unit"
                                :class="{'active-unit': isActiveStorageUnit(false)}"
                                @click="() => toggleStorageMeasurement(false)"
                            >
                                GB
                            </p>
                            <p
                                class="project-details__wrapper__container__limits__storagelimit-editing__unit"
                                :class="{'active-unit': isActiveStorageUnit(true)}"
                                @click="() => toggleStorageMeasurement(true)"
                            >
                                TB
                            </p>
                        </div>
                        <div class="project-details__wrapper__container__limits__storagelimit-editing">
                            <input
                                v-model="storageLimitValue"
                                class="project-details__wrapper__container__limits__storagelimit-editing__input"
                                placeholder="Enter a storage limit for your project"
                                type="number"
                                :maxlength="storageCharLimit"
                                :max="paidStorageLimit"
                                min="0"
                                @input="onStorageLimitInput"
                                @change="onStorageLimitInput"
                            >
                            <VButton
                                class="project-details__wrapper__container__limits__storagelimit-editing__save-button"
                                label="Save"
                                width="66px"
                                height="30px"
                                :on-press="onSaveStorageLimitButtonClick"
                            />
                        </div>
                    </div>
                    <p class="project-details__wrapper__container__label">Bandwidth Limit</p>
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
                    <div v-if="isBandwidthLimitEditing" class="project-details__wrapper__container__limits__bandwidthlimit-editing__section">
                        <div class="project-details__wrapper__container__limits__bandwidthlimit-editing__slider__wrapper">
                            <input
                                v-model="bandwidthLimitValue"
                                class="project-details__wrapper__container__limits__bandwidthlimit-editing__slider__input"
                                min="0"
                                :max="paidBandwidthLimit"
                                type="range"
                                @input="onBandwidthLimitInput"
                                @change="onBandwidthLimitInput"
                            >
                            <div class="project-details__wrapper__container__limits__bandwidthlimit-editing__slider__label-wrapper">
                                <p class="project-details__wrapper__container__limits__bandwidthlimit-editing__slider__label">0 {{ bandwidthMeasurementFormatted }}</p>
                                <p class="project-details__wrapper__container__limits__bandwidthlimit-editing__slider__label">{{ paidBandwidthLimit }}</p>
                            </div>
                        </div>
                        <div class="project-details__wrapper__container__limits__bandwidthlimit-editing__units-wrapper">
                            <p
                                class="project-details__wrapper__container__limits__bandwidthlimit-editing__unit"
                                :class="{'active-unit': isActiveBandwidthUnit(false)}"
                                @click="() => toggleBandwidthMeasurement(false)"
                            >
                                GB
                            </p>
                            <p
                                class="project-details__wrapper__container__limits__bandwidthlimit-editing__unit"
                                :class="{'active-unit': isActiveBandwidthUnit(true)}"
                                @click="() => toggleBandwidthMeasurement(true)"
                            >
                                TB
                            </p>
                        </div>
                        <div class="project-details__wrapper__container__limits__bandwidthlimit-editing">
                            <input
                                v-model="bandwidthLimitValue"
                                class="project-details__wrapper__container__limits__bandwidthlimit-editing__input"
                                placeholder="Enter a bandwidth limit for your project"
                                :max="paidBandwidthLimit"
                                min="0"
                                :maxlength="bandwidthCharLimit"
                                type="number"
                                @input="onBandwidthLimitInput"
                                @change="onBandwidthLimitInput"
                            >
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
    </div>
</template>

<script lang="ts">
import { Component, Vue, Prop } from 'vue-property-decorator';

import { Dimensions, Memory, Size } from '@/utils/bytesSize';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import {
    MAX_DESCRIPTION_LENGTH,
    MAX_NAME_LENGTH,
    Project,
    ProjectFields, ProjectLimits,
} from '@/types/projects';
import { MetaUtils } from '@/utils/meta';

import VButton from '@/components/common/VButton.vue';

// @vue/component
@Component({
    components: {
        VButton,
    },
})
export default class EditProjectDetails extends Vue {

    @Prop({ default: Dimensions.TB })
    public activeStorageMeasurement: string;
    @Prop({ default: Dimensions.TB })
    public activeBandwidthMeasurement: string;

    public isNameEditing = false;
    public isDescriptionEditing = false;
    public isStorageLimitEditing = false;
    public isBandwidthLimitEditing = false;
    public isPaidTier = false;
    public nameValue = '';
    public descriptionValue = '';
    public nameLength: number = MAX_NAME_LENGTH;
    public descriptionLength: number = MAX_DESCRIPTION_LENGTH;
    public storageLimitValue = 0;
    public bandwidthLimitValue = 0;

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

        if (this.$store.state.usersModule.user.paidTier) {
            this.isPaidTier = true;
        }

        try {
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, this.$store.getters.selectedProject.id);
        } catch (error) {
            return;
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
    public get bandwidthLimitFormatted(): string {
        return this.formattedValue(new Size(this.currentLimits.bandwidthLimit, 2));
    }

    public get storageLimitFormatted(): string {
        return this.formattedValue(new Size(this.currentLimits.storageLimit, 2));
    }

    /**
     * Returns formatted limit amount.
     */
    public get bandwidthLimitMeasurement(): string {
        return new Size(this.currentLimits.bandwidthLimit, 2).formattedBytes;
    }

    public get storageLimitMeasurement(): string {
        return new Size(this.currentLimits.storageLimit, 2).formattedBytes;
    }

    /**
     * Returns current input character limit.
     */
    public get bandwidthCharLimit(): number {

        if (this.activeBandwidthMeasurement == Dimensions.GB) {
            return 5;
        } else {
            return 2;
        }
    }

    public get storageCharLimit(): number {

        if (this.activeStorageMeasurement == Dimensions.GB) {
            return 5;
        } else {
            return 2;
        }
    }

    /**
     * Returns the current measurement that is being input.
     */
    public get storageMeasurementFormatted(): string {
        if (this.isStorageLimitEditing) {
            return this.activeStorageMeasurement;
        } else {
            return new Size(this.currentLimits.storageLimit, 2).label;
        }
    }

    public get bandwidthMeasurementFormatted(): string {
        if (this.isBandwidthLimitEditing) {
            return this.activeBandwidthMeasurement;
        } else {
            return new Size(this.currentLimits.bandwidthLimit, 2).label;
        }
    }

    /**
     * Gets current default limit for paid accounts.
     */
    public get paidBandwidthLimit(): number {
        if (this.activeBandwidthMeasurement == Dimensions.GB) {
            return this.toGB(this.getLimitValue(MetaUtils.getMetaContent('default-paid-bandwidth-limit')));
        } else {
            return this.getLimitValue(MetaUtils.getMetaContent('default-paid-bandwidth-limit'));
        }
    }

    public get paidStorageLimit(): number {
        if (this.activeStorageMeasurement == Dimensions.GB) {
            return this.toGB(this.getLimitValue(MetaUtils.getMetaContent('default-paid-storage-limit')));
        } else {
            return this.getLimitValue(MetaUtils.getMetaContent('default-paid-storage-limit'));
        }
    }

    /**
     * Convert value from GB to TB
     */
    public toTB(limitValue: number): number {
        return limitValue / 1000;
    }

    /**
     * Convert value from TB to GB
     */
    public toGB(limitValue: number): number {
        return limitValue * 1000;
    }

    /**
     * Get limit numeric value separated from included measurement
     */
    public getLimitValue(limit: string): number {
        return parseInt(limit.split(' ')[0]);
    }

    /**
     * Check if measurement unit is currently active.
     */
    public isActiveStorageUnit(isTB: boolean): boolean {
        if (isTB) {
            return this.activeStorageMeasurement == Dimensions.TB;
        } else {
            return this.activeStorageMeasurement == Dimensions.GB;
        }
    }

    public isActiveBandwidthUnit(isTB: boolean): boolean {
        if (isTB) {
            return this.activeBandwidthMeasurement == Dimensions.TB;
        } else {
            return this.activeBandwidthMeasurement == Dimensions.GB;
        }
    }

    /**
     * Toggles the current active unit, and makes input value measurement conversion.
     */
    public toggleStorageMeasurement(isTB: boolean): void {

        if (isTB) {
            this.activeStorageMeasurement = Dimensions.TB;
            this.storageLimitValue = this.toTB(this.storageLimitValue);
        } else {
            this.activeStorageMeasurement = Dimensions.GB;
            this.storageLimitValue = this.toGB(this.storageLimitValue);
        }
    }

    public toggleBandwidthMeasurement(isTB: boolean): void {

        if (isTB) {
            this.activeBandwidthMeasurement = Dimensions.TB;
            this.bandwidthLimitValue = this.toTB(this.bandwidthLimitValue);
        } else {
            this.activeBandwidthMeasurement = Dimensions.GB;
            this.bandwidthLimitValue = this.toGB(this.bandwidthLimitValue);
        }
    }

    /**
     * Triggers on limit input.
        Limits the input value based on default max limit and character limit.
     */
    public onStorageLimitInput(event: Event): void {
        const target = event.target as HTMLInputElement;
        const paidStorageCharLimit = this.paidStorageLimit.toString().length;

        if (target.value.length > paidStorageCharLimit) {
            const formattedLimit = target.value.slice(0, paidStorageCharLimit);
            this.storageLimitValue = parseInt(formattedLimit);
        } else if (parseInt(target.value) > this.paidStorageLimit) {
            this.storageLimitValue = this.paidStorageLimit;
        } else {
            this.storageLimitValue = parseInt(target.value);
        }
    }

    public onBandwidthLimitInput(event: Event): void {
        const target = event.target as HTMLInputElement;
        const paidBandwidthCharLimit = this.paidBandwidthLimit.toString().length;

        if (target.value.length > paidBandwidthCharLimit) {
            const formattedLimit = target.value.slice(0, paidBandwidthCharLimit);
            this.bandwidthLimitValue = parseInt(formattedLimit);
        } else if (parseInt(target.value) > this.paidBandwidthLimit) {
            this.bandwidthLimitValue = this.paidBandwidthLimit;
        } else {
            this.bandwidthLimitValue = parseInt(target.value);
        }
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
            return `${value.formattedBytes.replace(/\\.0+$/, '')} ${value.label}`;
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
            let storageLimitValue = this.storageLimitValue;

            if (this.activeStorageMeasurement == Dimensions.GB) {
                storageLimitValue = storageLimitValue * Number(Memory.GB);
            } else if (this.activeStorageMeasurement == Dimensions.TB) {
                storageLimitValue = storageLimitValue * Number(Memory.TB);
            }

            const updatedProject = new ProjectLimits(0, 0, storageLimitValue);
            await this.$store.dispatch(PROJECTS_ACTIONS.UPDATE_STORAGE_LIMIT, updatedProject);
        } catch (error) {
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
            let bandwidthLimitValue = this.bandwidthLimitValue;

            if (this.activeBandwidthMeasurement == Dimensions.GB) {
                bandwidthLimitValue = bandwidthLimitValue * Number(Memory.GB);
            } else if (this.activeBandwidthMeasurement == Dimensions.TB) {
                bandwidthLimitValue = bandwidthLimitValue * Number(Memory.TB);
            }

            const updatedProject = new ProjectLimits(bandwidthLimitValue);
            await this.$store.dispatch(PROJECTS_ACTIONS.UPDATE_BANDWIDTH_LIMIT, updatedProject);
        } catch (error) {
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

        const storageLimitUnit = new Size(this.currentLimits.storageLimit, 2).label;

        if (this.$store.state.usersModule.user.paidTier) {
            this.isStorageLimitEditing = !this.isStorageLimitEditing;

            if (this.activeStorageMeasurement == Dimensions.TB && storageLimitUnit !== Dimensions.TB) {
                this.storageLimitValue = this.toTB(parseInt(this.storageLimitMeasurement));
            } else if (this.activeStorageMeasurement == Dimensions.GB && storageLimitUnit !== Dimensions.GB) {
                this.storageLimitValue = parseInt(this.storageLimitMeasurement);
            } else {
                this.storageLimitValue = parseInt(this.storageLimitMeasurement);
            }
            this.activeStorageMeasurement = this.storageMeasurementFormatted;
        }
    }

    /**
     * Toggles project bandwidth limit editing state.
     */
    public toggleBandwidthLimitEditing(): void {
        const bandwidthLimitUnit = new Size(this.currentLimits.bandwidthLimit, 2).label;

        if (this.$store.state.usersModule.user.paidTier) {
            this.isBandwidthLimitEditing = !this.isBandwidthLimitEditing;

            if (this.activeBandwidthMeasurement == Dimensions.TB && bandwidthLimitUnit !== Dimensions.TB) {
                this.bandwidthLimitValue = this.toTB(parseInt(this.bandwidthLimitMeasurement));
            } else if (this.activeBandwidthMeasurement == Dimensions.GB && bandwidthLimitUnit !== Dimensions.GB) {
                this.bandwidthLimitValue = parseInt(this.bandwidthLimitMeasurement);
            } else {
                this.bandwidthLimitValue = parseInt(this.bandwidthLimitMeasurement);
            }
            this.activeBandwidthMeasurement = this.bandwidthMeasurementFormatted;
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
                margin: 0 0 20px;
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
                    margin: 0 0 35px;
                }

                &__label {
                    font-family: 'font_bold', sans-serif;
                    font-size: 16px;
                    line-height: 16px;
                    color: #384b65;
                    margin: 0 0 15px;
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
                        color: rgb(0 0 0 / 30%);
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
                &__limits__storagelimit-editing {
                    margin-bottom: 35px;
                }

                &__limits__storagelimit-editing,
                &__limits__bandwidthlimit-editing {
                    width: 180px;

                    &__section {
                        display: flex;
                        align-items: baseline;
                        position: relative;
                    }

                    &__input {
                        width: 100px;
                    }

                    &__slider {

                        &__wrapper {
                            width: 100%;
                        }

                        &__label-wrapper {
                            display: flex;
                            justify-content: space-between;
                            margin-top: 15px;
                            width: 95%;
                        }

                        &__label {
                            font-family: 'font_regular', sans-serif;
                            color: #768394;
                            font-size: 16px;
                        }

                        &__input {
                            width: 95%;
                            appearance: none;
                            height: 8px;
                            background: #f5f6fa;
                            outline: none;
                            transition: 0.2s;
                            transition: opacity 0.2s;
                            border: none;
                            border-radius: 6px;

                            &:hover {
                                opacity: 0.9;
                            }
                        }

                        &__input::-webkit-slider-thumb {
                            appearance: none;
                            width: 30px;
                            height: 30px;
                            background: #2582ff;
                            cursor: pointer;
                            border-radius: 50%;
                        }

                        &__input::-moz-range-thumb {
                            width: 30px;
                            height: 30px;
                            background: #2582ff;
                            cursor: pointer;
                            border-radius: 50%;
                        }
                    }

                    &__units-wrapper {
                        display: flex;
                        position: absolute;
                        right: 0;
                        bottom: 92px;
                    }

                    &__unit {
                        font-family: 'font_medium', sans-serif;
                        color: #afb7c1;
                        font-size: 16px;
                        margin-left: 10px;
                        padding: 5px 6px;
                        cursor: pointer;
                    }

                    &__unit.active-unit {
                        color: #2582ff;
                        background: #f5f6fa;
                        border-radius: 6px;
                    }
                }

                &__limits__bandwidthlimit-editing {

                    &__units-wrapper {
                        display: flex;
                        position: absolute;
                        right: 0;
                        bottom: 74px;
                    }
                }
            }
        }
    }
</style>

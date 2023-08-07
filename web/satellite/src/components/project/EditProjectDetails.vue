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
                        :is-white="true"
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
                        :is-white="true"
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
                            :is-white="true"
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
                    <p class="project-details__wrapper__container__label">Egress Limit</p>
                    <div v-if="!isBandwidthLimitEditing" class="project-details__wrapper__container__limits__bandwidthlimit-area">
                        <p class="project-details__wrapper__container__limits__bandwidthlimit-area__bandwidthlimit">{{ bandwidthLimitFormatted }}</p>
                        <VButton
                            label="Edit"
                            width="64px"
                            height="28px"
                            :on-press="toggleBandwidthLimitEditing"
                            :is-white="true"
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

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';

import { Dimensions, Memory, Size } from '@/utils/bytesSize';
import {
    MAX_DESCRIPTION_LENGTH,
    MAX_NAME_LENGTH,
    Project,
    ProjectFields, ProjectLimits,
} from '@/types/projects';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { RouteConfig } from '@/types/router';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import VButton from '@/components/common/VButton.vue';

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const router = useRouter();

const activeStorageMeasurement = ref<string>(Dimensions.TB);
const activeBandwidthMeasurement = ref<string>(Dimensions.TB);
const isNameEditing = ref<boolean>(false);
const isDescriptionEditing = ref<boolean>(false);
const isStorageLimitEditing = ref<boolean>(false);
const isBandwidthLimitEditing = ref<boolean>(false);
const isPaidTier = ref<boolean>(false);
const nameValue = ref<string>('');
const descriptionValue = ref<string>('');
const nameLength = ref<number>(MAX_NAME_LENGTH);
const descriptionLength = ref<number>(MAX_DESCRIPTION_LENGTH);
const storageLimitValue = ref<number>(0);
const bandwidthLimitValue = ref<number>(0);

/**
 * Returns selected project from store.
 */
const storedProject = computed((): Project => {
    return projectsStore.state.selectedProject;
});

/**
 * Returns current limits from store.
 */
const currentLimits = computed((): ProjectLimits => {
    return projectsStore.state.currentLimits;
});

/**
 * Returns displayed project description on UI.
 */
const displayedDescription = computed((): string => {
    return storedProject.value.description ?
        storedProject.value.description :
        'No description yet. Please enter some information if any.';
});

/**
 * Returns formatted limit amount.
 */
const bandwidthLimitFormatted = computed((): string => {
    return formattedValue(new Size(currentLimits.value.bandwidthLimit, 2));
});

const storageLimitFormatted = computed((): string => {
    return formattedValue(new Size(currentLimits.value.storageLimit, 2));
});

/**
 * Returns formatted limit amount.
 */
const bandwidthLimitMeasurement = computed((): string => {
    return new Size(currentLimits.value.bandwidthLimit, 2).formattedBytes;
});

const storageLimitMeasurement = computed((): string => {
    return new Size(currentLimits.value.storageLimit, 2).formattedBytes;
});

/**
 * Returns current input character limit.
 */
const bandwidthCharLimit = computed((): number => {
    if (activeBandwidthMeasurement.value === Dimensions.GB) {
        return 5;
    } else {
        return 2;
    }
});

const storageCharLimit = computed((): number => {
    if (activeStorageMeasurement.value === Dimensions.GB) {
        return 5;
    } else {
        return 2;
    }
});

/**
 * Returns the current measurement that is being input.
 */
const storageMeasurementFormatted = computed((): string => {
    if (isStorageLimitEditing.value) {
        return activeStorageMeasurement.value;
    } else {
        return new Size(currentLimits.value.storageLimit, 2).label;
    }
});

const bandwidthMeasurementFormatted = computed((): string => {
    if (isBandwidthLimitEditing.value) {
        return activeBandwidthMeasurement.value;
    } else {
        return new Size(currentLimits.value.bandwidthLimit, 2).label;
    }
});

/**
 * Gets current default limit for paid accounts.
 */
const paidBandwidthLimit = computed((): number => {
    const limitVal = getLimitValue(configStore.state.config.defaultPaidBandwidthLimit);
    const maxLimit = Math.max(currentLimits.value.bandwidthLimit / Memory.TB, limitVal);
    if (activeBandwidthMeasurement.value === Dimensions.GB) {
        return toGB(maxLimit);
    }
    return maxLimit;
});

const paidStorageLimit = computed((): number => {
    const limitVal = getLimitValue(configStore.state.config.defaultPaidStorageLimit);
    const maxLimit = Math.max(currentLimits.value.storageLimit / Memory.TB, limitVal);
    if (activeStorageMeasurement.value === Dimensions.GB) {
        return toGB(maxLimit);
    }
    return maxLimit;
});

/**
 * Triggers on name input.
 */
function onNameInput(event: Event): void {
    const target = event.target as HTMLInputElement;
    if (target.value.length < MAX_NAME_LENGTH) {
        nameValue.value = target.value;
        return;
    }

    nameValue.value = target.value.slice(0, MAX_NAME_LENGTH);
}

/**
 * Triggers on description input.
 */
function onDescriptionInput(event: Event): void {
    const target = event.target as HTMLInputElement;
    if (target.value.length < MAX_DESCRIPTION_LENGTH) {
        descriptionValue.value = target.value;
        return;
    }

    descriptionValue.value = target.value.slice(0, MAX_DESCRIPTION_LENGTH);
}

/**
 * Convert value from GB to TB
 */
function toTB(limitValue: number): number {
    return limitValue / 1000;
}

/**
 * Convert value from TB to GB
 */
function toGB(limitValue: number): number {
    return limitValue * 1000;
}

/**
 * Get limit numeric value separated from included measurement
 */
function getLimitValue(limit: string): number {
    return parseInt(limit.split(' ')[0]);
}

/**
 * Check if measurement unit is currently active.
 */
function isActiveStorageUnit(isTB: boolean): boolean {
    if (isTB) {
        return activeStorageMeasurement.value === Dimensions.TB;
    } else {
        return activeStorageMeasurement.value === Dimensions.GB;
    }
}

function isActiveBandwidthUnit(isTB: boolean): boolean {
    if (isTB) {
        return activeBandwidthMeasurement.value === Dimensions.TB;
    } else {
        return activeBandwidthMeasurement.value === Dimensions.GB;
    }
}

/**
 * Toggles the current active unit, and makes input value measurement conversion.
 */
function toggleStorageMeasurement(isTB: boolean): void {
    if (isTB) {
        activeStorageMeasurement.value = Dimensions.TB;
        storageLimitValue.value = toTB(storageLimitValue.value);
    } else {
        activeStorageMeasurement.value = Dimensions.GB;
        storageLimitValue.value = toGB(storageLimitValue.value);
    }
}

function toggleBandwidthMeasurement(isTB: boolean): void {
    if (isTB) {
        activeBandwidthMeasurement.value = Dimensions.TB;
        bandwidthLimitValue.value = toTB(bandwidthLimitValue.value);
    } else {
        activeBandwidthMeasurement.value = Dimensions.GB;
        bandwidthLimitValue.value = toGB(bandwidthLimitValue.value);
    }
}

/**
 * Triggers on limit input.
 Limits the input value based on default max limit and character limit.
 */
function onStorageLimitInput(event: Event): void {
    const target = event.target as HTMLInputElement;
    const paidStorageCharLimit = paidStorageLimit.value.toString().length;

    if (target.value.length > paidStorageCharLimit) {
        const formattedLimit = target.value.slice(0, paidStorageCharLimit);
        storageLimitValue.value = parseInt(formattedLimit);
    } else if (parseInt(target.value) > paidStorageLimit.value) {
        storageLimitValue.value = paidStorageLimit.value;
    } else {
        storageLimitValue.value = parseInt(target.value);
    }
}

function onBandwidthLimitInput(event: Event): void {
    const target = event.target as HTMLInputElement;
    const paidBandwidthCharLimit = paidBandwidthLimit.value.toString().length;

    if (target.value.length > paidBandwidthCharLimit) {
        const formattedLimit = target.value.slice(0, paidBandwidthCharLimit);
        bandwidthLimitValue.value = parseInt(formattedLimit);
    } else if (parseInt(target.value) > paidBandwidthLimit.value) {
        bandwidthLimitValue.value = paidBandwidthLimit.value;
    } else {
        bandwidthLimitValue.value = parseInt(target.value);
    }
}

/**
 * Formats value to needed form and returns it.
 */
function formattedValue(value: Size): string {
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
async function onSaveNameButtonClick(): Promise<void> {
    try {
        const updatedProject = new ProjectFields(nameValue.value, '');
        updatedProject.checkName();

        await projectsStore.updateProjectName(updatedProject);
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.EDIT_PROJECT_DETAILS);
        return;
    }

    toggleNameEditing();
    analyticsStore.eventTriggered(AnalyticsEvent.PROJECT_NAME_UPDATED);
    notify.success('Project name updated successfully!');
}

/**
 * Updates project description.
 */
async function onSaveDescriptionButtonClick(): Promise<void> {
    try {
        const updatedProject = new ProjectFields('', descriptionValue.value);
        await projectsStore.updateProjectDescription(updatedProject);
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.EDIT_PROJECT_DETAILS);
        return;
    }

    toggleDescriptionEditing();
    analyticsStore.eventTriggered(AnalyticsEvent.PROJECT_DESCRIPTION_UPDATED);
    notify.success('Project description updated successfully!');
}

/**
 * Updates project storage limit.
 */
async function onSaveStorageLimitButtonClick(): Promise<void> {
    try {
        let storageLimit = storageLimitValue.value;

        if (activeStorageMeasurement.value === Dimensions.GB) {
            storageLimit = storageLimit * Number(Memory.GB);
        } else if (activeStorageMeasurement.value === Dimensions.TB) {
            storageLimit = storageLimit * Number(Memory.TB);
        }

        const updatedProject = new ProjectLimits(0, 0, storageLimit);
        await projectsStore.updateProjectStorageLimit(updatedProject);
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.EDIT_PROJECT_DETAILS);
        return;
    }

    toggleStorageLimitEditing();
    analyticsStore.eventTriggered(AnalyticsEvent.PROJECT_STORAGE_LIMIT_UPDATED);
    notify.success('Project storage limit updated successfully!');
}

/**
 * Updates project bandwidth limit.
 */
async function onSaveBandwidthLimitButtonClick(): Promise<void> {
    try {
        let bandwidthLimit = bandwidthLimitValue.value;

        if (activeBandwidthMeasurement.value === Dimensions.GB) {
            bandwidthLimit = bandwidthLimit * Number(Memory.GB);
        } else if (activeBandwidthMeasurement.value === Dimensions.TB) {
            bandwidthLimit = bandwidthLimit * Number(Memory.TB);
        }

        const updatedProject = new ProjectLimits(bandwidthLimit);
        await projectsStore.updateProjectBandwidthLimit(updatedProject);
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.EDIT_PROJECT_DETAILS);
        return;
    }

    toggleBandwidthLimitEditing();
    analyticsStore.eventTriggered(AnalyticsEvent.PROJECT_BANDWIDTH_LIMIT_UPDATED);
    notify.success('Project egress limit updated successfully!');
}

/**
 * Toggles project name editing state.
 */
function toggleNameEditing(): void {
    isNameEditing.value = !isNameEditing.value;
    nameValue.value = storedProject.value.name;
}

/**
 * Toggles project description editing state.
 */
function toggleDescriptionEditing(): void {
    isDescriptionEditing.value = !isDescriptionEditing.value;
    descriptionValue.value = storedProject.value.description;
}

/**
 * Toggles project storage limit editing state.
 */
function toggleStorageLimitEditing(): void {
    const storageLimitUnit = new Size(currentLimits.value.storageLimit, 2).label;

    if (usersStore.state.user.paidTier) {
        isStorageLimitEditing.value = !isStorageLimitEditing.value;

        if (activeStorageMeasurement.value === Dimensions.TB && storageLimitUnit !== Dimensions.TB) {
            storageLimitValue.value = toTB(parseInt(storageLimitMeasurement.value));
        } else if (activeStorageMeasurement.value === Dimensions.GB && storageLimitUnit !== Dimensions.GB) {
            storageLimitValue.value = parseInt(storageLimitMeasurement.value);
        } else {
            storageLimitValue.value = parseInt(storageLimitMeasurement.value);
        }
        activeStorageMeasurement.value = storageMeasurementFormatted.value;
    }
}

/**
 * Toggles project bandwidth limit editing state.
 */
function toggleBandwidthLimitEditing(): void {
    const bandwidthLimitUnit = new Size(currentLimits.value.bandwidthLimit, 2).label;

    if (usersStore.state.user.paidTier) {
        isBandwidthLimitEditing.value = !isBandwidthLimitEditing.value;

        if (activeBandwidthMeasurement.value === Dimensions.TB && bandwidthLimitUnit !== Dimensions.TB) {
            bandwidthLimitValue.value = toTB(parseInt(bandwidthLimitMeasurement.value));
        } else if (activeBandwidthMeasurement.value === Dimensions.GB && bandwidthLimitUnit !== Dimensions.GB) {
            bandwidthLimitValue.value = parseInt(bandwidthLimitMeasurement.value);
        } else {
            bandwidthLimitValue.value = parseInt(bandwidthLimitMeasurement.value);
        }
        activeBandwidthMeasurement.value = bandwidthMeasurementFormatted.value;
    }
}

/**
 * Redirects to previous route.
 */
function onBackClick(): void {
    const PREVIOUS_ROUTE_NUMBER = -1;
    router.go(PREVIOUS_ROUTE_NUMBER);
}

/**
 * Lifecycle hook after initial render.
 * Fetches project limits and paid tier status.
 */
onMounted(async (): Promise<void> => {
    const projectID = projectsStore.state.selectedProject.id;
    if (!projectID) return;

    if (projectsStore.state.selectedProject.ownerId !== usersStore.state.user.id) {
        await router.replace(configStore.state.config.allProjectsDashboard ? RouteConfig.AllProjectsDashboard : RouteConfig.ProjectDashboard.path);
        return;
    }

    projectsStore.$onAction(({ name, after }) => {
        if (name === 'selectProject') {
            after((_) => {
                if (projectsStore.state.selectedProject.ownerId !== usersStore.state.user.id) {
                    router.replace(RouteConfig.ProjectDashboard.path);
                }
            });
        }
    });

    if (usersStore.state.user.paidTier) {
        isPaidTier.value = true;
    }

    try {
        await projectsStore.getProjectLimits(projectID);
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.EDIT_PROJECT_DETAILS);
    }
});
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

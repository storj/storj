// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <div class="modal__header">
                    <LimitIcon />
                    <h1 class="modal__header__title">{{ activeLimit }} Limit</h1>
                </div>
                <div class="modal__functional">
                    <div class="modal__functional__limits">
                        <div class="modal__functional__limits__wrap">
                            <p class="modal__functional__limits__wrap__label">Set {{ activeLimit }} Limit</p>
                            <div class="modal__functional__limits__wrap__inputs">
                                <input
                                    v-model="limitValue"
                                    type="number"
                                    :min="0"
                                    :max="isBandwidthUpdating ? paidBandwidthLimit : paidStorageLimit"
                                    @input="setLimitValue"
                                >
                                <select
                                    :value="activeMeasurement"
                                    @change="setActiveMeasurement"
                                >
                                    <option
                                        v-for="option in measurementOptions"
                                        :key="option"
                                        :value="option"
                                    >
                                        {{ option }}
                                    </option>
                                </select>
                            </div>
                        </div>
                        <div class="modal__functional__limits__wrap">
                            <p class="modal__functional__limits__wrap__label">Available {{ activeLimit }}</p>
                            <div class="modal__functional__limits__wrap__inputs">
                                <input
                                    :value="isBandwidthUpdating ? paidBandwidthLimit : paidStorageLimit"
                                    disabled
                                >
                                <select
                                    :value="activeMeasurement"
                                    @change="setActiveMeasurement"
                                >
                                    <option
                                        v-for="option in measurementOptions"
                                        :key="option"
                                        :value="option"
                                    >
                                        {{ option }}
                                    </option>
                                </select>
                            </div>
                        </div>
                    </div>
                    <div class="modal__functional__range">
                        <div class="modal__functional__range__labels">
                            <p>0 {{ activeMeasurement }}</p>
                            <p>
                                {{ isBandwidthUpdating ? paidBandwidthLimit.toLocaleString() : paidStorageLimit.toLocaleString() }} {{ activeMeasurement }}
                            </p>
                        </div>
                        <input
                            ref="rangeInput"
                            v-model="limitValue"
                            min="0"
                            :max="isBandwidthUpdating ? paidBandwidthLimit : paidStorageLimit"
                            type="range"
                            @input="setLimitValue"
                        >
                    </div>
                </div>
                <p class="modal__info">
                    If you need more storage,
                    <span
                        class="modal__info__link"
                        rel="noopener noreferrer"
                        @click="openRequestLimitModal"
                    >
                        request limit increase.
                    </span>
                </p>
                <div class="modal__buttons">
                    <VButton
                        label="Cancel"
                        :on-press="closeModal"
                        width="100%"
                        height="48px"
                        font-size="14px"
                        border-radius="10px"
                        :is-disabled="isLoading"
                        is-white
                    />
                    <VButton
                        label="Save"
                        :on-press="onSave"
                        width="100%"
                        height="48px"
                        font-size="14px"
                        border-radius="10px"
                        :is-disabled="isLoading"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, onMounted, ref } from 'vue';

import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useNotify } from '@/utils/hooks';
import { LimitToChange, ProjectLimits } from '@/types/projects';
import { Dimensions, Memory } from '@/utils/bytesSize';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useLoading } from '@/composables/useLoading';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { MODALS } from '@/utils/constants/appStatePopUps';

import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';

import LimitIcon from '@/../static/images/modals/limit.svg';

const analyticsStore = useAnalyticsStore();
const appStore = useAppStore();
const projectsStore = useProjectsStore();
const configStore = useConfigStore();
const notify = useNotify();

const { isLoading, withLoading } = useLoading();

const activeLimit = ref<LimitToChange>(LimitToChange.Storage);
const activeMeasurement = ref<string>(Dimensions.TB);
const limitValue = ref<number>(0);
const rangeInput = ref<HTMLInputElement>();

/**
 * Returns current limits from store.
 */
const currentLimits = computed((): ProjectLimits => {
    return projectsStore.state.currentLimits;
});

/**
 * Returns current default bandwidth limit for paid accounts.
 */
const paidBandwidthLimit = computed((): number => {
    const limitVal = getLimitValue(configStore.state.config.defaultPaidBandwidthLimit);
    const maxLimit = Math.max(currentLimits.value.bandwidthLimit / Memory.TB, limitVal);
    if (activeMeasurement.value === Dimensions.GB) {
        return toGB(maxLimit);
    }
    return maxLimit;
});

/**
 * Returns current default storage limit for paid accounts.
 */
const paidStorageLimit = computed((): number => {
    const limitVal = getLimitValue(configStore.state.config.defaultPaidStorageLimit);
    const maxLimit = Math.max(currentLimits.value.storageLimit / Memory.TB, limitVal);
    if (activeMeasurement.value === Dimensions.GB) {
        return toGB(maxLimit);
    }
    return maxLimit;
});

/**
 * Returns dimensions dropdown options.
 */
const measurementOptions = computed((): string[] => {
    return [Dimensions.GB, Dimensions.TB];
});

/**
 * Indicates if bandwidth limit is updating.
 */
const isBandwidthUpdating = computed((): boolean => {
    return activeLimit.value === LimitToChange.Bandwidth;
});

function openRequestLimitModal(): void {
    appStore.updateActiveModal(MODALS.requestProjectLimitIncrease);
}

/**
 * Sets active dimension and recalculates limit values.
 */
function setActiveMeasurement(event: Event): void {
    const target = event.target as HTMLSelectElement;

    if (target.value === Dimensions.TB) {
        activeMeasurement.value = Dimensions.TB;
        limitValue.value = toTB(limitValue.value);
        updateTrackColor();
        return;
    }

    activeMeasurement.value = Dimensions.GB;
    limitValue.value = toGB(limitValue.value);
    updateTrackColor();
}

/**
 * Sets limit value from inputs.
 */
function setLimitValue(event: Event): void {
    const target = event.target as HTMLInputElement;

    const paidCharLimit = isBandwidthUpdating.value ?
        paidBandwidthLimit.value.toString().length :
        paidStorageLimit.value.toString().length;

    if (target.value.length > paidCharLimit) {
        const formattedLimit = target.value.slice(0, paidCharLimit);
        limitValue.value = parseFloat(formattedLimit);
        return;
    }

    if (activeLimit.value === LimitToChange.Bandwidth && parseFloat(target.value) > paidBandwidthLimit.value) {
        limitValue.value = paidBandwidthLimit.value;
        return;
    }

    if (activeLimit.value === LimitToChange.Storage && parseFloat(target.value) > paidStorageLimit.value) {
        limitValue.value = paidStorageLimit.value;
        return;
    }

    limitValue.value = parseFloat(target.value);

    updateTrackColor();
}

/**
 * Get limit numeric value separated from included measurement
 */
function getLimitValue(limit: string): number {
    return parseInt(limit.split(' ')[0]);
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
 * Closes modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}

/**
 * Updates range input's track color depending on current active value.
 */
function updateTrackColor(): void {
    if (!rangeInput.value) {
        return;
    }

    const min = parseFloat(rangeInput.value.min);
    const max = parseFloat(rangeInput.value.max);
    const value = parseFloat(rangeInput.value.value);

    const thumbPosition = (value - min) / (max - min);
    const greenWidth = thumbPosition * 100;

    rangeInput.value.style.background = `linear-gradient(to right, #00ac26 ${greenWidth}%, #d8dee3 ${greenWidth}%)`;
}

/**
 * Updates desired limit.
 */
async function onSave(): Promise<void> {
    await withLoading(async () => {
        try {
            let limit = limitValue.value;
            if (activeMeasurement.value === Dimensions.GB) {
                limit = limit * Number(Memory.GB);
            } else if (activeMeasurement.value === Dimensions.TB) {
                limit = limit * Number(Memory.TB);
            }

            if (isBandwidthUpdating.value) {
                const updatedProject = new ProjectLimits(limit);
                await projectsStore.updateProjectBandwidthLimit(updatedProject);

                analyticsStore.eventTriggered(AnalyticsEvent.PROJECT_BANDWIDTH_LIMIT_UPDATED);
                notify.success('Project egress limit updated successfully!');
            } else {
                const updatedProject = new ProjectLimits(0, 0, limit);
                await projectsStore.updateProjectStorageLimit(updatedProject);

                analyticsStore.eventTriggered(AnalyticsEvent.PROJECT_STORAGE_LIMIT_UPDATED);
                notify.success('Project storage limit updated successfully!');
            }

            closeModal();
        } catch (error) {
            notify.error(error.message, AnalyticsErrorEventSource.CHANGE_PROJECT_LIMIT_MODAL);
        }
    });
}

onBeforeMount(() => {
    activeLimit.value = appStore.state.activeChangeLimit;

    if (isBandwidthUpdating.value) {
        limitValue.value = currentLimits.value.bandwidthLimit / Memory.TB;
        return;
    }

    limitValue.value = currentLimits.value.storageLimit / Memory.TB;
});

onMounted(() => {
    updateTrackColor();
});
</script>

<style scoped lang="scss">
.modal {
    padding: 32px;
    font-family: 'font_regular', sans-serif;

    @media screen and (width <= 375px) {
        padding: 32px 16px;
    }

    &__header {
        display: flex;
        align-items: center;
        padding-bottom: 16px;
        margin-bottom: 16px;
        border-bottom: 1px solid var(--c-grey-2);

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 24px;
            line-height: 31px;
            letter-spacing: -0.02em;
            color: var(--c-black);
            margin-left: 16px;
        }
    }

    &__functional {
        padding: 16px;
        background: var(--c-grey-1);
        border: 1px solid var(--c-grey-2);
        border-radius: 16px;

        &__limits {
            display: flex;
            align-items: center;
            column-gap: 16px;

            &__wrap {
                @media screen and (width <= 475px) {
                    width: calc(50% - 8px);
                }

                &__label {
                    font-weight: 500;
                    font-size: 14px;
                    line-height: 20px;
                    color: var(--c-blue-6);
                    text-align: left;
                    margin-bottom: 8px;
                }

                &__inputs {
                    display: flex;
                    align-items: center;
                    border: 1px solid var(--c-grey-4);
                    border-radius: 8px;

                    input,
                    select {
                        border: none;
                        font-family: 'font_bold', sans-serif;
                        font-size: 14px;
                        line-height: 20px;
                        color: var(--c-grey-6);
                        background-color: #fff;
                    }

                    input {
                        padding: 9px 13px;
                        box-sizing: border-box;
                        width: 100px;
                        border-radius: 8px 0 0 8px;

                        @media screen and (width <= 475px) {
                            padding-right: 0;
                            width: calc(100% - 54px);
                        }

                        &:disabled {
                            background-color: var(--c-grey-2);
                        }
                    }

                    select {
                        box-sizing: border-box;
                        min-width: 54px;
                        padding: 9px 0 9px 13px;
                        border-radius: 0 8px 8px 0;
                    }
                }
            }
        }

        &__range {
            padding: 16px;
            border: 1px solid var(--c-grey-3);
            border-radius: 8px;
            margin-top: 16px;
            background-color: var(--c-white);

            &__labels {
                display: flex;
                align-items: center;
                justify-content: space-between;
                margin-bottom: 10px;

                p {
                    font-family: 'font_bold', sans-serif;
                    font-size: 14px;
                    line-height: 14px;
                    color: var(--c-grey-6);
                }
            }
        }
    }

    &__info {
        font-weight: 500;
        font-size: 14px;
        line-height: 20px;
        color: var(--c-blue-6);
        margin-top: 16px;
        text-align: left;

        &__link {
            text-decoration: underline !important;
            text-underline-position: under;
            color: var(--c-blue-6);
            cursor: pointer;

            &:visited {
                color: var(--c-blue-6);
            }
        }
    }

    &__buttons {
        border-top: 1px solid var(--c-grey-2);
        margin-top: 16px;
        padding-top: 24px;
        display: flex;
        align-items: center;
        column-gap: 16px;
    }
}

input[type='range'] {
    width: 100%;
    cursor: pointer;
    appearance: none;
    border: none;
    border-radius: 4px;
}

input[type='range']::-webkit-slider-thumb {
    appearance: none;
    margin-top: -4px;
    width: 16px;
    height: 16px;
    background: var(--c-white);
    border: 1px solid var(--c-green-5);
    cursor: col-resize;
    border-radius: 50%;
    background-image: url('../../../static/images/modals/burger.png');
    background-repeat: no-repeat;
    background-size: 10px 7px;
    background-position: center;
}

input[type='range']::-moz-range-thumb {
    appearance: none;
    margin-top: -4px;
    width: 16px;
    height: 16px;
    background: var(--c-white);
    border: 1px solid var(--c-green-5);
    cursor: col-resize;
    border-radius: 50%;
    background-image: url('../../../static/images/modals/burger.png');
    background-repeat: no-repeat;
    background-size: 10px 7px;
    background-position: center;
}

input[type='range']::-webkit-slider-runnable-track {
    width: 100%;
    height: 8px;
    cursor: pointer;
    border-radius: 4px;
}

input[type='range']::-moz-range-track {
    width: 100%;
    height: 8px;
    cursor: pointer;
    border-radius: 4px;
}
</style>

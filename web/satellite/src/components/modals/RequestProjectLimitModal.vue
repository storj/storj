// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <div class="modal__header">
                    <LimitIcon />
                    <h1 class="modal__header__title">{{ activeLimit }} Limit Request</h1>
                </div>

                <p class="modal__info">
                    Request a {{ activeLimit }} limit increase for this project.
                </p>

                <div class="modal__functional">
                    <div class="modal__functional__limits">
                        <div class="modal__functional__limits__wrap">
                            <p class="modal__functional__limits__wrap__label">{{ activeLimit }} Limit</p>
                            <div class="modal__functional__limits__wrap__inputs">
                                <input
                                    :value="currentActiveLimit"
                                    disabled
                                >
                                <div class="modal__functional__limits__wrap__inputs__selector">
                                    <div tabindex="0" class="modal__functional__limits__wrap__inputs__selector__content" @keyup.enter="() => toggleSelector('current')" @click.stop="() => toggleSelector('current')">
                                        <span v-if="activeMeasurement" class="modal__functional__limits__wrap__inputs__selector__content__label">{{ activeMeasurement }}</span>
                                        <span v-else class="modal__functional__limits__wrap__inputs__selector__content__label">Measurement</span>
                                        <arrow-down-icon class="modal__functional__limits__wrap__inputs__selector__content__arrow" :class="{ open: curLimitMeasurementOpen }" />
                                    </div>
                                    <div v-if="curLimitMeasurementOpen" v-click-outside="closeSelector" class="modal__functional__limits__wrap__inputs__selector__dropdown">
                                        <div
                                            v-for="(option, index) in measurementOptions"
                                            :key="index"
                                            tabindex="0"
                                            class="modal__functional__limits__wrap__inputs__selector__dropdown__item"
                                            :class="{ selected: activeMeasurement === option }"
                                            @click.stop="() => setActiveMeasurement(option)"
                                            @keyup.enter="() => setActiveMeasurement(option)"
                                        >
                                            <span class="modal__functional__limits__wrap__inputs__selector__dropdown__item__label">{{ option }}</span>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                        <div class="modal__functional__limits__wrap">
                            <p class="modal__functional__limits__wrap__label">Requested Limit</p>
                            <div class="modal__functional__limits__wrap__inputs">
                                <input
                                    v-model="limitValue"
                                    type="number"
                                    :min="0"
                                    @input="setLimitValue"
                                >
                                <div class="modal__functional__limits__wrap__inputs__selector">
                                    <div tabindex="0" class="modal__functional__limits__wrap__inputs__selector__content" @keyup.enter="() => toggleSelector('requested')" @click.stop="() => toggleSelector('requested')">
                                        <span v-if="activeMeasurement" class="modal__functional__limits__wrap__inputs__selector__content__label">{{ activeMeasurement }}</span>
                                        <span v-else class="modal__functional__limits__wrap__inputs__selector__content__label">Measurement</span>
                                        <arrow-down-icon class="modal__functional__limits__wrap__inputs__selector__content__arrow" :class="{ open: reqLimitMeasurementOpen }" />
                                    </div>
                                    <div v-if="reqLimitMeasurementOpen" v-click-outside="closeSelector" class="modal__functional__limits__wrap__inputs__selector__dropdown">
                                        <div
                                            v-for="(option, index) in measurementOptions"
                                            :key="index"
                                            tabindex="0"
                                            class="modal__functional__limits__wrap__inputs__selector__dropdown__item"
                                            :class="{ selected: activeMeasurement === option }"
                                            @click.stop="() => setActiveMeasurement(option)"
                                            @keyup.enter="() => setActiveMeasurement(option)"
                                        >
                                            <span class="modal__functional__limits__wrap__inputs__selector__dropdown__item__label">{{ option }}</span>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
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
                        label="Send"
                        :on-press="sendRequest"
                        width="100%"
                        height="48px"
                        font-size="14px"
                        border-radius="10px"
                        :is-disabled="isSendDisabled"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref } from 'vue';

import ArrowDownIcon from '../../../static/images/common/dropIcon.svg';

import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useNotify } from '@/utils/hooks';
import { LimitToChange, ProjectLimits } from '@/types/projects';
import { Dimensions, Memory } from '@/utils/bytesSize';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useLoading } from '@/composables/useLoading';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { APP_STATE_DROPDOWNS } from '@/utils/constants/appStatePopUps';

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
 * Returns current limit for which an increase is being requested.
 */
const currentActiveLimit = computed((): number => {
    return isRequestingBandwidth.value ? paidBandwidthLimit.value : paidStorageLimit.value;
});

/**
 * Returns dimensions dropdown options.
 */
const measurementOptions = computed((): Dimensions[] => {
    return [Dimensions.GB, Dimensions.TB];
});

/**
 * Indicates if bandwidth limit is updating.
 */
const isRequestingBandwidth = computed((): boolean => {
    return activeLimit.value === LimitToChange.Bandwidth;
});

/**
 * whether the measurement dropdown is open.
 */
const curLimitMeasurementOpen = computed((): boolean => {
    return appStore.state.activeDropdown === APP_STATE_DROPDOWNS.SIZE_MEASUREMENT_SELECTOR;
});

/**
 * whether the measurement dropdown is open.
 */
const reqLimitMeasurementOpen = computed((): boolean => {
    return appStore.state.activeDropdown === APP_STATE_DROPDOWNS.REQUESTED_SIZE_MEASUREMENT_SELECTOR;
});

/**
 * whether the send button should be disabled.
 */
const isSendDisabled = computed((): boolean => {
    return limitValue.value === 0 || limitValue.value === currentActiveLimit.value || isLoading.value;
});

/**
 * Opens the measurement dropdown.
 */
function toggleSelector(which: string) {
    if (curLimitMeasurementOpen.value || curLimitMeasurementOpen.value) {
        appStore.closeDropdowns();
    } else if (which === 'current') {
        appStore.toggleActiveDropdown(APP_STATE_DROPDOWNS.SIZE_MEASUREMENT_SELECTOR);
    } else if (which === 'requested') {
        appStore.toggleActiveDropdown(APP_STATE_DROPDOWNS.REQUESTED_SIZE_MEASUREMENT_SELECTOR);
    }
}

/**
 * Closes the measurement dropdown.
 */
function closeSelector() {
    appStore.closeDropdowns();
}

/**
 * Sets active dimension and recalculates limit values.
 */
function setActiveMeasurement(measurement: Dimensions): void {
    closeSelector();
    if (activeMeasurement.value === measurement) {
        return;
    }
    if (measurement === Dimensions.TB) {
        activeMeasurement.value = Dimensions.TB;
        limitValue.value = toTB(limitValue.value);
        return;
    }

    activeMeasurement.value = Dimensions.GB;
    limitValue.value = toGB(limitValue.value);
}

/**
 * Sets limit value from inputs.
 */
function setLimitValue(event: Event): void {
    const target = event.target as HTMLInputElement;

    limitValue.value = parseFloat(target.value);
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
 * Updates desired limit.
 */
async function sendRequest(): Promise<void> {
    await withLoading(async () => {
        try {
            let limit = limitValue.value;
            if (activeMeasurement.value === Dimensions.GB) {
                limit = limit * Number(Memory.GB);
            } else if (activeMeasurement.value === Dimensions.TB) {
                limit = limit * Number(Memory.TB);
            }
            await projectsStore.requestLimitIncrease(activeLimit.value, limit);
            notify.success('', `
                <span class="message-title">Your request for limits increase has been submitted.</span>
                <span class="message-info">Limit increases may take up to 3 business days to be reflected in your limits.</span>
            `);
            closeModal();
        } catch (error) {
            notify.error(error.message, AnalyticsErrorEventSource.REQUEST_PROJECT_LIMIT_MODAL);
        }
    });
}

onBeforeMount(() => {
    activeLimit.value = appStore.state.activeChangeLimit;

    if (isRequestingBandwidth.value) {
        limitValue.value = currentLimits.value.bandwidthLimit / Memory.TB;
        return;
    }

    limitValue.value = currentLimits.value.storageLimit / Memory.TB;
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
                    background-color: var(--c-white);

                    input {
                        padding: 9px 13px;
                        box-sizing: border-box;
                        width: 100px;
                        border-radius: 8px 0 0 8px;
                        border: none;
                        font-family: 'font_bold', sans-serif;
                        font-size: 14px;
                        line-height: 20px;
                        color: var(--c-grey-6);

                        @media screen and (width <= 475px) {
                            padding-right: 0;
                            width: calc(100% - 54px);
                        }

                        &:disabled {
                            background-color: var(--c-grey-2);
                        }
                    }

                    &__selector {
                        width: 65px;
                        border-radius: 6px;
                        box-sizing: border-box;

                        &__content {
                            display: flex;
                            align-items: center;
                            justify-content: space-between;
                            position: relative;
                            padding: 5px 14px;

                            &__label {
                                font-family: 'font_bold', sans-serif;
                                font-size: 14px;
                                line-height: 20px;
                                color: var(--c-grey-6);
                                cursor: default;
                            }

                            &__arrow {
                                transition-duration: 0.5s;

                                &.open {
                                    transform: rotate(180deg) scaleX(-1);
                                }
                            }
                        }

                        &__dropdown {
                            position: absolute;
                            bottom: 70px;
                            background: var(--c-white);
                            z-index: 999;
                            box-sizing: border-box;
                            box-shadow: 0 -2px 16px rgb(0 0 0 / 10%);
                            border-radius: 8px;
                            border: 1px solid var(--c-grey-2);
                            width: 60px;

                            &__item {
                                padding: 10px;

                                &__label {
                                    cursor: default;
                                }

                                &.selected {
                                    background: var(--c-grey-1);
                                }

                                &:first-of-type {
                                    border-top-right-radius: 8px;
                                    border-top-left-radius: 8px;
                                }

                                &:last-of-type {
                                    border-bottom-right-radius: 8px;
                                    border-bottom-left-radius: 8px;
                                }

                                &:hover {
                                    background: var(--c-grey-2);
                                }
                            }
                        }
                    }
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
        margin-bottom: 16px;
        text-align: left;

        &__link {
            text-decoration: underline !important;
            text-underline-position: under;
            color: var(--c-blue-6);

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
</style>

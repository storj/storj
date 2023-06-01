// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="modal">
        <div class="modal__header" :class="{'custom-radius': !isExpanded}" @click="toggleExpanded">
            <div class="modal__header__left">
                <div class="modal__header__left__info">
                    <p class="modal__header__left__info__title">{{ statusLabel }}</p>
                    <p class="modal__header__left__info__remaining">{{ remainingTimeString }}</p>
                </div>
                <div v-if="!isClosable" class="modal__header__left__track">
                    <div class="modal__header__left__track__fill" :style="progressStyle" />
                </div>
            </div>
            <div class="modal__header__right">
                <ArrowIcon class="modal__header__right__arrow" :class="{rotated: isExpanded}" />
                <CloseIcon v-if="isClosable" class="modal__header__right__close" @click="closeModal" />
            </div>
        </div>
        <div v-if="isExpanded" class="modal__items">
            <div v-for="item in uploading" :key="item.Key">
                <UploadItem :item="item" />
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';

import { UploadingBrowserObject, UploadingStatus, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useAppStore } from '@/store/modules/appStore';
import { Duration } from '@/utils/time';

import UploadItem from '@/components/modals/objectUpload/UploadItem.vue';

import ArrowIcon from '@/../static/images/modals/objectUpload/arrow.svg';
import CloseIcon from '@/../static/images/modals/objectUpload/close.svg';

const obStore = useObjectBrowserStore();
const appStore = useAppStore();

const isExpanded = ref<boolean>(false);
const startDate = ref<number>(Date.now());
const remainingTimeString = ref<string>('');
const interval = ref<NodeJS.Timer>();

/**
 * Returns uploading objects from store.
 */
const uploading = computed((): UploadingBrowserObject[] => {
    return obStore.state.uploading;
});

/**
 * Returns uploading objects with InProgress status.
 */
const objectsInProgress = computed((): UploadingBrowserObject[] => {
    return uploading.value.filter(f => f.status === UploadingStatus.InProgress);
});

/**
 * Indicates if modal is closable.
 */
const isClosable = computed((): boolean => {
    return !objectsInProgress.value.length;
});

/**
 * Returns header's status label.
 */
const statusLabel = computed((): string => {
    if (isClosable.value) {
        let status = 'Uploading completed';

        const failedUploads = uploading.value.filter(f => f.status === UploadingStatus.Failed);
        if (failedUploads.length > 0) {
            status += ` (${failedUploads.length} failed`;
        }

        const cancelledUploads = uploading.value.filter(f => f.status === UploadingStatus.Cancelled);
        if (cancelledUploads.length > 0) {
            status += `, (${cancelledUploads.length} cancelled`;
        }

        if (!failedUploads.length && !cancelledUploads.length) {
            return status;
        }

        return `${status})`;
    }

    if (uploading.value.length === 1) {
        return 'Uploading 1 item';
    }

    return `Uploading ${uploading.value.length} items`;
});

/**
 * Returns progress bar style.
 */
const progressStyle = computed((): Record<string, string> => {
    const progress = uploading.value.reduce((total: number, item: UploadingBrowserObject) => {
        total += item.progress || 0;
        return total;
    }, 0) / uploading.value.length;

    return {
        width: `${progress}%`,
    };
});

/**
 * Calculates remaining seconds.
 */
function calculateRemainingTime(): void {
    const progress = uploading.value.reduce((total: number, item: UploadingBrowserObject) => {
        if (item.progress && item.progress !== 100) {
            total += item.progress;
        }
        return total;
    }, 0);

    const remainingProgress = 100 - progress;
    const averageProgressPerNanosecond = progress / ((Date.now() - startDate.value) * 1000000);
    const remainingNanoseconds = remainingProgress / averageProgressPerNanosecond;
    if (!isFinite(remainingNanoseconds) || remainingNanoseconds < 0) {
        return;
    }

    remainingTimeString.value = new Duration(remainingNanoseconds).remainingFormatted;
}

/**
 * Toggles expanded state.
 */
function toggleExpanded(): void {
    isExpanded.value = !isExpanded.value;
}

/**
 * Closes modal.
 */
function closeModal(): void {
    obStore.clearUploading();
    appStore.setUploadingModal(false);
}

/**
 * Starts interval for recalculating remaining time.
 */
function startInterval(): void {
    const int = setInterval(() => {
        if (isClosable.value) {
            clearInterval(int);
            interval.value = undefined;
            remainingTimeString.value = '';
            return;
        }

        calculateRemainingTime();
    }, 2000); // recalculate every 2 seconds.

    interval.value = int;
}

watch(() => objectsInProgress.value.length, () => {
    if (!interval.value) {
        startDate.value = Date.now();
        startInterval();
    }
});

onMounted(() => {
    startInterval();
});
</script>

<style scoped lang="scss">
.modal {
    position: fixed;
    right: 24px;
    bottom: 24px;
    width: 500px;
    max-width: 500px;
    border-radius: 8px;
    font-family: 'font_regular', sans-serif;

    @media screen and (width <= 650px) {
        max-width: unset;
        width: unset;
        left: 126px;
    }

    @media screen and (width <= 500px) {
        left: 24px;
    }

    &__header {
        background-color: var(--c-grey-10);
        padding: 16px;
        cursor: pointer;
        border-radius: 8px 8px 0 0;
        display: flex;
        align-items: center;
        justify-content: space-between;

        &__left {
            box-sizing: border-box;
            margin-right: 24px;
            width: 100%;

            &__info {
                display: flex;
                align-items: center;
                justify-content: space-between;

                &__title {
                    font-family: 'font_medium', sans-serif;
                    font-size: 14px;
                    line-height: 20px;
                    color: var(--c-white);
                }

                &__remaining {
                    font-size: 14px;
                    line-height: 20px;
                    color: var(--c-white);
                    opacity: 0.7;
                }
            }

            &__track {
                margin-top: 10px;
                width: 100%;
                height: 8px;
                border-radius: 4px;
                position: relative;
                background-color: var(--c-grey-11);

                &__fill {
                    position: absolute;
                    top: 0;
                    left: 0;
                    bottom: 0;
                    background-color: var(--c-blue-1);
                    border-radius: 4px;
                    max-width: 100%;
                }
            }
        }

        &__right {
            display: flex;
            align-items: center;

            &__arrow {
                transition: all 0.3s ease-out;
            }

            &__close {
                margin-left: 30px;

                :deep(path) {
                    fill: var(--c-white);
                }
            }
        }
    }

    &__items {
        border: 1px solid var(--c-grey-3);
        border-radius: 0 0 8px 8px;
        max-height: 185px;
        overflow-y: auto;
    }
}

.rotated {
    transform: rotate(180deg);
}

.custom-radius {
    border-radius: 8px;
}
</style>

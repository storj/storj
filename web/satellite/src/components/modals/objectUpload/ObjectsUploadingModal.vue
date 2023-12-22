// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="modal">
        <div class="modal__header" :class="{'custom-radius': !isExpanded}" @click="toggleExpanded">
            <div class="modal__header__left">
                <div class="modal__header__left__info">
                    <div class="modal__header__left__info__cont">
                        <component :is="icon" v-if="icon" :class="{ close: icon === FailedIcon }" />
                        <p class="modal__header__left__info__cont__title">{{ statusLabel }}</p>
                    </div>
                    <div class="modal__header__left__info__right">
                        <p class="modal__header__left__info__right__remaining">{{ remainingTimeString }}</p>
                        <p v-if="!isClosable" class="modal__header__left__info__right__cancel" @click.stop="cancelAll">Cancel</p>
                    </div>
                </div>
                <div v-if="!isClosable" class="modal__header__left__track">
                    <div v-if="progress" class="modal__header__left__track__fill" :style="progressStyle" />
                    <div v-else class="modal__header__left__track__indeterminate" />
                </div>
            </div>
            <div class="modal__header__right">
                <ArrowIcon class="modal__header__right__arrow" :class="{rotated: isExpanded}" />
                <CloseIcon v-if="isClosable" class="modal__header__right__close" @click="closeModal" />
            </div>
        </div>
        <div v-if="isExpanded" class="modal__items">
            <UploadItem
                v-for="item in uploading"
                :key="item.Key"
                :class="{ modal__items__completed: item.status == UploadingStatus.Finished }"
                :item="item"
                @click="() => showFile(item)"
            />
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch, Component } from 'vue';

import { UploadingBrowserObject, UploadingStatus, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useAppStore } from '@/store/modules/appStore';
import { Duration } from '@/utils/time';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useConfigStore } from '@/store/modules/configStore';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';

import UploadItem from '@/components/modals/objectUpload/UploadItem.vue';

import ArrowIcon from '@/../static/images/modals/objectUpload/arrow.svg';
import CloseIcon from '@/../static/images/modals/objectUpload/close.svg';
import CompleteIcon from '@/../static/images/modals/objectUpload/complete.svg';
import FailedIcon from '@/../static/images/modals/objectUpload/failed.svg';

const obStore = useObjectBrowserStore();
const appStore = useAppStore();
const bucketsStore = useBucketsStore();
const notify = useNotify();
const projectsStore = useProjectsStore();
const config = useConfigStore();

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
 * Returns what icon should be displayed in the header.
 */
const icon = computed((): string => {
    if (!isClosable.value) return '';
    if (uploading.value.some(f => f.status === UploadingStatus.Finished)) return CompleteIcon;
    if (uploading.value.some(f => f.status === UploadingStatus.Failed)) return FailedIcon;
    return '';
});

/**
 * Returns header's status label.
 */
const statusLabel = computed((): string => {
    if (!uploading.value.length) return 'No items to upload';
    let inProgress = 0, finished = 0, failed = 0, cancelled = 0;
    uploading.value.forEach(u => {
        switch (u.status) {
        case UploadingStatus.InProgress:
            inProgress++;
            break;
        case UploadingStatus.Failed:
            failed++;
            break;
        case UploadingStatus.Cancelled:
            cancelled++;
            break;
        default:
            finished++;
        }
    });

    if (failed === uploading.value.length) return 'Uploading failed';
    if (cancelled === uploading.value.length) return 'Uploading cancelled';
    if (inProgress) return `Uploading ${inProgress} item${inProgress > 1 ? 's' : ''}`;

    const statuses = [
        failed ? `${failed} failed` : '',
        cancelled ? `${cancelled} cancelled` : '',
    ].filter(s => s).join(', ');

    return `Uploading completed${statuses ? ` (${statuses})` : ''}`;
});

/**
 * Returns upload progress.
 */
const progress = computed((): number => {
    return uploading.value.reduce((total: number, item: UploadingBrowserObject) => {
        total += item.progress || 0;
        return total;
    }, 0) / uploading.value.length;
});

/**
 * Returns progress bar style.
 */
const progressStyle = computed((): Record<string, string> => {
    return {
        width: `${progress.value}%`,
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
 * Cancels all uploads in progress.
 */
function cancelAll(): void {
    objectsInProgress.value.forEach(item => {
        try {
            obStore.cancelUpload(item.Key);
        } catch (error) {
            notify.error(`Unable to cancel upload for '${item.Key}'. ${error.message}`, AnalyticsErrorEventSource.OBJECTS_UPLOAD_MODAL);
        }
    });
}

/**
 * Opens the object preview.
 */
function showFile(item: UploadingBrowserObject): void {
    if (item.status !== UploadingStatus.Finished) return;

    obStore.setObjectPathForModal(item.Key);

    if (config.state.config.galleryViewEnabled) {
        appStore.setGalleryView(true);
    } else {
        appStore.updateActiveModal(MODALS.objectDetails);
    }
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
    isExpanded.value = false;
    appStore.setUploadingModal(false);
    obStore.clearUploading();
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

watch(() => projectsStore.state.selectedProject, (value, oldValue) => {
    if (value.id === oldValue.id || !appStore.state.isUploadingModal) {
        return;
    }
    closeModal();
});

watch(() =>  bucketsStore.state.fileComponentBucketName, (value, oldValue) => {
    if (value === oldValue || !appStore.state.isUploadingModal) {
        return;
    }
    closeModal();
});

watch(() => objectsInProgress.value.length, () => {
    if (!interval.value) {
        startDate.value = Date.now();
        startInterval();
    }
});

/**
 * Close the modal if nothing is uploading.
 */
watch(uploading, (value, oldValue) => {
    if (value.length === oldValue.length) {
        return;
    }

    if (!value.length) {
        closeModal();
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
    filter: drop-shadow(0 7px 20px rgb(0 0 0 / 15%));

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
        gap: 24px;

        &__left {
            box-sizing: border-box;
            width: 100%;

            &__info {
                display: flex;
                align-items: center;
                justify-content: space-between;

                &__cont {
                    display: flex;
                    align-items: center;
                    gap: 11px;

                    svg {
                        width: 24px;
                        height: 24px;
                    }

                    &__title {
                        font-size: 14px;
                        line-height: 20px;
                        color: var(--c-white);
                    }
                }

                &__right {
                    display: flex;
                    align-items: center;
                    gap: 17px;
                    font-size: 14px;
                    line-height: 20px;
                    color: var(--c-white);

                    &__remaining {
                        opacity: 0.7;
                        text-align: right;
                    }

                    &__cancel {
                        cursor: pointer;

                        &:hover {
                            text-decoration: underline;
                        }
                    }
                }
            }

            &__track {
                margin-top: 10px;
                width: 100%;
                height: 8px;
                border-radius: 4px;
                position: relative;
                background-color: var(--c-grey-11);
                overflow: hidden;

                &__fill {
                    position: absolute;
                    top: 0;
                    left: 0;
                    bottom: 0;
                    background-color: var(--c-blue-1);
                    border-radius: 4px;
                    max-width: 100%;
                }

                &__indeterminate {
                    position: absolute;
                    top: 0;
                    left: 0;
                    bottom: 0;
                    background-color: var(--c-blue-1);
                    border-radius: 4px;
                    max-width: 100%;
                    width: 50%;
                    animation: indeterminate-progress-bar;
                    animation-duration: 2s;
                    animation-iteration-count: infinite;
                }

                @keyframes indeterminate-progress-bar {

                    from {
                        left: -50%;
                    }

                    to {
                        left: 100%;
                    }
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

        &__completed {
            cursor: pointer;

            &:hover {
                background-color: var(--c-grey-1);
            }

            &:active {
                background-color: var(--c-grey-2);
            }
        }
    }
}

.rotated {
    transform: rotate(180deg);
}

.custom-radius {
    border-radius: 8px;
}
</style>

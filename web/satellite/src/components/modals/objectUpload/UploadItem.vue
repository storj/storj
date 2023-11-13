// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="item">
        <div class="item__left">
            <div class="item__left__icon">
                <component :is="icon" />
            </div>
            <p class="item__left__name" :title="item.Key">{{ item.Key }}</p>
        </div>
        <div class="item__right">
            <template v-if="item.status === UploadingStatus.InProgress">
                <div class="item__right__track">
                    <div v-if="item.progress" class="item__right__track__fill" :style="progressStyle" />
                    <div v-else class="item__right__track__indeterminate" />
                </div>
                <CloseIcon class="item__right__cancel" @click="cancelUpload" />
            </template>
            <p v-if="item.status === UploadingStatus.Cancelled" class="item__right__cancelled">Upload cancelled</p>
            <CheckIcon v-if="item.status === UploadingStatus.Finished" />
            <template v-if="item.status === UploadingStatus.Failed">
                <p class="item__right__failed">{{ item.failedMessage }}</p>
                <FailedIcon />
                <VInfo v-if="item.failedMessage === FailedUploadMessage.TooBig" class="item__right__info">
                    <template #icon>
                        <InfoIcon />
                    </template>
                    <template #message>
                        <p class="item__right__info__message">
                            Use Command Line Interface to drop files more than 30 GB.
                            <a
                                class="item__right__info__message__link"
                                href="https://docs.storj.io/dcs/getting-started/quickstart-uplink-cli/prerequisites"
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                More information
                            </a>
                        </p>
                    </template>
                </VInfo>
                <p v-else class="item__right__retry" @click="retryUpload">Retry</p>
            </template>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import {
    UploadingBrowserObject,
    UploadingStatus,
    FailedUploadMessage,
    useObjectBrowserStore,
} from '@/store/modules/objectBrowserStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { ObjectType } from '@/utils/objectIcon';

import VInfo from '@/components/common/VInfo.vue';

import CloseIcon from '@/../static/images/modals/objectUpload/close.svg';
import CheckIcon from '@/../static/images/modals/objectUpload/check.svg';
import FailedIcon from '@/../static/images/modals/objectUpload/failed.svg';
import InfoIcon from '@/../static/images/modals/objectUpload/info.svg';

const obStore = useObjectBrowserStore();
const notify = useNotify();

const props = defineProps<{
    item: UploadingBrowserObject
}>();

/**
 * Returns appropriate icon for provided object.
 */
const icon = computed((): string => ObjectType.findIcon(ObjectType.findType(props.item.Key)));

/**
 * Returns progress bar style.
 */
const progressStyle = computed((): Record<string, string> => {
    return {
        width: props.item.progress ? `${props.item.progress}%` : '0%',
    };
});

/**
 * Retries failed upload.
 */
async function retryUpload(): Promise<void> {
    try {
        await obStore.retryUpload(props.item.Key);
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.OBJECTS_UPLOAD_MODAL);
    }
}

/**
 * Cancels active upload.
 */
function cancelUpload(): void {
    try {
        obStore.cancelUpload(props.item.Key);
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.OBJECTS_UPLOAD_MODAL);
    }
}
</script>

<style scoped lang="scss">
.item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    border-top: 1px solid var(--c-grey-3);
    padding: 14px 20px;
    font-family: 'font_regular', sans-serif;
    background-color: var(--c-white);

    @media screen and (width <= 450px) {
        padding: 14px;
    }

    &:last-of-type {
        border-radius: 0 0 8px 8px;
    }

    &__left {
        display: flex;
        align-items: center;
        max-width: 56%;

        @media screen and (width <= 450px) {
            max-width: 40%;
        }

        &__icon {
            min-width: 32px;
            width: 32px;
            height: 32px;
            background-color: var(--c-white);
            border: 1px solid var(--c-grey-2);
            border-radius: 8px;
            margin-right: 12px;
            display: flex;
            align-items: center;
            justify-content: center;

            @media screen and (width <= 550px) {
                display: none;
            }
        }

        &__name {
            font-size: 14px;
            line-height: 20px;
            color: var(--c-grey-9);
            overflow: hidden;
            white-space: nowrap;
            text-overflow: ellipsis;
        }
    }

    &__right {
        display: flex;
        align-items: center;
        margin-left: 20px;

        svg {
            width: 20px;
            height: 20px;
        }

        &__track {
            min-width: 130px;
            height: 6px;
            border-radius: 3px;
            position: relative;
            margin-right: 34px;
            background-color: var(--c-blue-1);
            overflow: hidden;

            &__fill {
                position: absolute;
                top: 0;
                left: 0;
                bottom: 0;
                background-color: var(--c-blue-3);
                border-radius: 3px;
                max-width: 100%;
            }

            &__indeterminate {
                position: absolute;
                top: 0;
                left: 0;
                bottom: 0;
                background-color: var(--c-blue-3);
                border-radius: 3px;
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

        &__cancel {
            cursor: pointer;
        }

        &__cancelled {
            font-size: 14px;
            line-height: 20px;
            color: var(--c-grey-5);
            text-align: right;
        }

        &__failed {
            font-size: 14px;
            line-height: 20px;
            color: var(--c-red-4);
            margin-right: 8px;
        }

        &__retry {
            font-size: 14px;
            line-height: 20px;
            color: var(--c-blue-3);
            margin-left: 18px;
            cursor: pointer;

            &:hover {
                text-decoration: underline;
            }
        }

        &__info {
            cursor: pointer;
            max-height: 20px;
            margin-left: 18px;

            &__message {
                font-size: 14px;
                line-height: 20px;
                text-align: center;
                color: var(--c-black);

                &__link {
                    color: var(--c-blue-3);

                    &:visited {
                        color: var(--c-blue-3);
                    }
                }
            }
        }
    }
}

:deep(.info__box) {
    width: 290px;
    left: calc(50% - 265px);
    top: calc(100% - 85px);
    cursor: default;
    filter: none;
    transform: rotate(-180deg);
}

:deep(.info__box__message) {
    border-radius: 4px;
    padding: 10px 8px;
    transform: rotate(-180deg);
    border: 1px solid var(--c-grey-5);
}

:deep(.info__box__arrow) {
    width: 10px;
    height: 10px;
    margin-bottom: -3px;
}
</style>

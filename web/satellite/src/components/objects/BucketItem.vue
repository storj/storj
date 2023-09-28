// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table-item
        :item="itemToRender"
        :on-click="onClick"
        :on-primary-click="onClick"
        :show-guide="shouldShowGuide"
        :hide-guide="hideGuidePermanently"
        item-type="bucket"
    >
        <template #options>
            <th v-click-outside="closeDropdown" :class="{active: isDropdownOpen}" class="bucket-item__functional options overflow-visible" @click.stop="openDropdown(dropdownKey)">
                <dots-icon />
                <div v-if="isDropdownOpen" class="bucket-item__functional__dropdown">
                    <div class="bucket-item__functional__dropdown__item" @click.stop="onDetailsClick">
                        <details-icon />
                        <p class="bucket-item__functional__dropdown__item__label">View Bucket Details</p>
                    </div>
                    <div class="bucket-item__functional__dropdown__item" @click.stop="onShareClick">
                        <share-icon />
                        <p class="bucket-item__functional__dropdown__item__label">Share Bucket</p>
                    </div>
                    <div class="bucket-item__functional__dropdown__item delete" @click.stop="onDeleteClick">
                        <delete-icon />
                        <p class="bucket-item__functional__dropdown__item__label">Delete Bucket</p>
                    </div>
                </div>
            </th>
        </template>
    </table-item>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { RouteConfig } from '@/types/router';
import { Bucket } from '@/types/buckets';
import { LocalData } from '@/utils/localData';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useResize } from '@/composables/resize';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { ShareType } from '@/types/browser';

import TableItem from '@/components/common/TableItem.vue';

import DeleteIcon from '@/../static/images/objects/delete.svg';
import DetailsIcon from '@/../static/images/objects/details.svg';
import ShareIcon from '@/../static/images/objects/share.svg';
import DotsIcon from '@/../static/images/objects/dots.svg';

const appStore = useAppStore();
const bucketsStore = useBucketsStore();
const router = useRouter();
const route = useRoute();
const { screenWidth } = useResize();

const props = withDefaults(defineProps<{
    itemData: Bucket;
    openDropdown: (key: number) => void;
    onClick: (bucket: string) => void;
    isDropdownOpen: boolean;
    showGuide: boolean;
    dropdownKey: number;
}>(), {
    itemData: () => new Bucket(),
    openDropdown: (_: number) => {},
    onClick: (_: string) => {},
    isDropdownOpen: false,
    showGuide: true,
    dropdownKey: -1,
});

const isGuideShown = ref<boolean>(true);

const shouldShowGuide = computed((): boolean => {
    return props.showGuide && isGuideShown.value;
});

/**
 * Returns formatted date.
 */
const formattedDate = computed((): string => {
    return props.itemData.since.toLocaleString('en-US', { day: '2-digit', month: 'numeric', year: 'numeric' }) || '';
});

const itemToRender = computed((): { [key: string]: string | string[] } => {
    if (screenWidth.value > 875) return {
        name: props.itemData.name,
        storage: `${props.itemData.storage.toFixed(2)}GB`,
        bandwidth: `${props.itemData.egress.toFixed(2)}GB`,
        objects: props.itemData.objectCount.toString(),
        segments: props.itemData.segmentCount.toString(),
        date: formattedDate.value,
    };

    return { info: [
        props.itemData.name,
        `Storage ${props.itemData.storage.toFixed(2)}GB`,
        `Egress ${props.itemData.egress.toFixed(2)}GB`,
        `Objects ${props.itemData.objectCount.toString()}`,
        `Segments ${props.itemData.segmentCount.toString()}`,
        `Created ${formattedDate.value}`,
    ] };
});

/**
 * Permanently hide the upload guide
 */
function hideGuidePermanently(): void {
    isGuideShown.value = false;
    LocalData.setBucketGuideHidden();
}

/**
 * Closes dropdown.
 */
function closeDropdown(): void {
    props.openDropdown(-1);
}

/**
 * Holds on delete click logic.
 */
function onDeleteClick(): void {
    bucketsStore.setBucketToDelete(props.itemData.name);
    appStore.updateActiveModal(MODALS.deleteBucket);
    closeDropdown();
}

/**
 * Redirects to bucket details page.
 */
function onDetailsClick(): void {
    router.push({
        name: RouteConfig.Buckets.with(RouteConfig.BucketsDetails).name,
        query: {
            bucketName: props.itemData.name,
            backRoute: route.name as string || '',
        },
    });

    closeDropdown();
}

/**
 * Opens the Share modal for this bucket.
 */
function onShareClick(): void {
    bucketsStore.setFileComponentBucketName(props.itemData.name);
    appStore.setShareModalType(ShareType.Bucket);

    if (bucketsStore.state.promptForPassphrase) {
        appStore.updateActiveModal(MODALS.enterBucketPassphrase);
        bucketsStore.setEnterPassphraseCallback((): void => {
            appStore.updateActiveModal(MODALS.share);
        });
        return;
    }
    appStore.updateActiveModal(MODALS.share);
}

onMounted((): void => {
    isGuideShown.value = !LocalData.getBucketGuideHidden();
});
</script>

<style scoped lang="scss">
    .bucket-item {

        &__functional {
            padding: 0 10px;
            cursor: pointer;
            pointer-events: auto;
            position: relative;

            &__dropdown {
                position: absolute;
                top: 25px;
                right: 15px;
                background: #fff;
                box-shadow: 0 20px 34px rgb(10 27 44 / 28%);
                border-radius: 6px;
                width: 255px;
                z-index: 100;
                overflow: hidden;

                &__item {
                    display: flex;
                    align-items: center;
                    padding: 20px 25px;
                    width: calc(100% - 50px);

                    &__label {
                        margin: 0 0 0 10px;
                    }

                    &:hover {
                        background-color: #f4f5f7;
                        font-family: 'font_medium', sans-serif;
                        color: var(--c-blue-3);

                        svg :deep(path) {
                            fill: var(--c-blue-3);
                        }
                    }
                }

                &__item.delete {
                    border-top: 1px solid #e5e7eb;
                }
            }

            &__message {
                position: absolute;
                top: 80%;
                width: 25rem;
                display: flex;
                flex-direction: column;
                align-items: flex-start;
                transform: translateX(-100%);
                background-color: var(--c-blue-3);
                text-align: center;
                border-radius: 8px;
                box-sizing: border-box;
                padding: 20px;
                z-index: 1001;

                @media screen and (width <= 320px) {
                    transform: translateX(-80%);
                }

                @media screen and (width <= 375px) and (width >= 350px) {
                    transform: translateX(-88%);
                }

                &:after {
                    content: '';
                    position: absolute;
                    bottom: 100%;
                    left: 10%;
                    border-width: 5px;
                    border-style: solid;
                    border-color: var(--c-blue-3) transparent transparent;
                    transform: rotate(180deg);

                    @media screen and (width <= 550px) {
                        left: 45%;
                    }
                }

                &__title {
                    font-weight: 400;
                    font-size: 12px;
                    line-height: 18px;
                    color: white;
                    opacity: 0.5;
                    margin-bottom: 4px;
                }

                &__content {
                    color: white;
                    font-weight: 500;
                    font-size: 15px;
                    line-height: 24px;
                    margin-bottom: 8px;
                    white-space: initial;
                    text-align: start;
                }

                &__actions {
                    display: flex;
                    justify-content: flex-end;
                    align-items: center;
                    width: 100%;

                    &__primary {
                        padding: 6px 12px;
                        border-radius: 8px;
                        background-color: white;
                        margin-left: 10px;

                        &__label {
                            font-weight: 700;
                            font-size: 13px;
                            line-height: 20px;
                            color: var(--c-blue-3);
                        }
                    }
                }
            }
        }
    }

    :deep(.primary) {
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
    }

    @media screen and (width <= 1400px) {

        :deep(th) {
            max-width: 25rem;
        }
    }

    @media screen and (width <= 1100px) {

        :deep(th) {
            max-width: 20rem;
        }
    }

    @media screen and (width <= 1000px) {

        :deep(th) {
            max-width: 15rem;
        }
    }

    @media screen and (width <= 940px) {

        :deep(th) {
            max-width: 10rem;
        }
    }
</style>

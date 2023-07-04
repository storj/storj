// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div
        class="bucket-settings-nav"
        @click.stop.prevent="isDropdownOpen = !isDropdownOpen"
        @mouseenter="isHoveredOver = true"
        @mouseleave="isHoveredOver = false"
    >
        <div class="bucket-settings-nav__button">
            <GearIcon class="bucket-settings-nav__button__icon" :class="{active: isHoveredOver || isDropdownOpen}" />
            <arrow-down-icon class="bucket-settings-nav__arrow" :class="{active: isDropdownOpen, hovered: isHoveredOver}" />
        </div>
        <div v-show="isDropdownOpen" v-click-outside="closeDropdown" class="bucket-settings-nav__dropdown">
            <!-- TODO: add other options and place objects popup in common place and trigger from store -->
            <div class="bucket-settings-nav__dropdown__item" @click.stop="onDetailsClick">
                <details-icon class="bucket-settings-nav__dropdown__item__icon" />
                <p class="bucket-settings-nav__dropdown__item__label">View Bucket Details</p>
            </div>
            <div v-if="filesCount" class="bucket-settings-nav__dropdown__item" @click.stop="onShareBucketClick">
                <share-icon class="bucket-settings-nav__dropdown__item__icon" />
                <p class="bucket-settings-nav__dropdown__item__label">Share bucket</p>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { RouteConfig } from '@/types/router';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';
import { useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { ShareType } from '@/types/browser';

import ArrowDownIcon from '@/../static/images/common/dropIcon.svg';
import DetailsIcon from '@/../static/images/objects/details.svg';
import ShareIcon from '@/../static/images/objects/share.svg';
import GearIcon from '@/../static/images/common/gearIcon.svg';

const obStore = useObjectBrowserStore();
const appStore = useAppStore();
const router = useRouter();
const route = useRoute();

const props = defineProps<{
    bucketName: string,
}>();

const isDropdownOpen = ref(false);
const isHoveredOver = ref(false);

/**
 * Returns files amount from store.
 */
const filesCount = computed((): number => {
    return obStore.sortedFiles.length;
});

function closeDropdown(): void {
    if (!isDropdownOpen.value) return;

    isDropdownOpen.value = false;
}

/**
 * Redirects to bucket details page.
 */
function onDetailsClick(): void {
    router.push({
        name: RouteConfig.BucketsDetails.name,
        query: {
            bucketName: props.bucketName,
            backRoute: route.name as string || '',
        },
    });

    isDropdownOpen.value = false;
}

/**
 * Toggles share bucket modal.
 */
function onShareBucketClick(): void {
    appStore.setShareModalType(ShareType.Bucket);
    appStore.updateActiveModal(MODALS.share);
    isDropdownOpen.value = false;
}
</script>

<style scoped lang="scss">
.bucket-settings-nav {
    position: relative;
    font-family: 'font_regular', sans-serif;
    font-style: normal;
    display: flex;
    align-items: center;
    padding: 14px 18px;
    height: 44px;
    width: 78px;
    box-sizing: border-box;
    font-weight: normal;
    font-size: 14px;
    line-height: 19px;
    color: #1b2533;
    cursor: pointer;
    background: white;
    border: 1px solid var(--c-grey-3);
    border-radius: 8px;

    &:hover,
    &:active,
    &:focus {
        border: 1px solid var(--c-blue-3);
    }

    &__button {
        display: flex;
        align-items: center;
        justify-content: space-between;
        cursor: pointer;

        svg:first-of-type {
            margin-right: 10px;
        }

        &__icon {
            transition-duration: 0.5s;
        }

        &__icon.active {

            :deep(path) {
                fill: var(--c-blue-3);
            }
        }
    }

    &__dropdown {
        position: absolute;
        top: 50px;
        right: 0;
        display: flex;
        flex-direction: column;
        justify-content: center;
        background: #fff;
        box-shadow: 0 20px 34px rgb(10 27 44 / 28%);
        border-radius: 6px;
        width: 255px;
        z-index: 100;
        transition-duration: 0.5s;
        overflow: hidden;

        &__item {
            box-sizing: border-box;
            display: flex;
            align-items: center;
            padding: 17px 21px;
            width: 100%;

            &__label {
                margin: 0 0 0 17px;
            }

            &:hover {
                background-color: #f4f5f7;
                font-family: 'font_medium', sans-serif;
                color: var(--c-blue-3);

                :deep(path) {
                    fill: var(--c-blue-3);
                }
            }
        }
    }

    &__arrow {
        transition-duration: 0.5s;
    }

    &__arrow.active {
        transform: rotate(180deg) scaleX(-1);
    }

    &__arrow.hovered {

        :deep(path) {
            fill: var(--c-blue-3);
        }
    }
}
</style>

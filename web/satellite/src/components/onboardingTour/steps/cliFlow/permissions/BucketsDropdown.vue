// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-click-outside="closeDropdown" class="buckets-dropdown">
        <div class="buckets-dropdown__container">
            <p class="buckets-dropdown__container__all" @click.stop="clearSelectedBuckets">
                All
            </p>
            <label class="buckets-dropdown__container__search">
                <input
                    v-model="bucketSearch"
                    class="buckets-dropdown__container__search__input"
                    placeholder="Search buckets"
                    type="text"
                >
            </label>
            <div
                v-for="(name, index) in bucketsList"
                :key="index"
                class="buckets-dropdown__container__choices"
            >
                <div
                    class="buckets-dropdown__container__choices__item"
                    :class="{ selected: isNameSelected(name) }"
                    @click.stop="toggleBucketSelection(name)"
                >
                    <div class="buckets-dropdown__container__choices__item__left">
                        <div class="check-icon">
                            <SelectionIcon v-if="isNameSelected(name)" />
                        </div>
                        <p class="buckets-dropdown__container__choices__item__left__label">{{ name }}</p>
                    </div>
                    <UnselectIcon
                        v-if="isNameSelected(name)"
                        class="buckets-dropdown__container__choices__item__unselect-icon"
                    />
                </div>
            </div>
            <p v-if="!bucketsList.length" class="buckets-dropdown__container__no-buckets">
                No Buckets
            </p>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { useAppStore } from '@/store/modules/appStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';

import SelectionIcon from '@/../static/images/accessGrants/selection.svg';
import UnselectIcon from '@/../static/images/accessGrants/unselect.svg';

const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const agStore = useAccessGrantsStore();

const bucketSearch = ref<string>('');

/**
 * Returns stored bucket names list filtered by search string.
 */
const bucketsList = computed((): string[] => {
    const NON_EXIST_INDEX = -1;
    const buckets: string[] = bucketsStore.state.allBucketNames;

    return buckets.filter((name: string) => {
        return name.indexOf(bucketSearch.value.toLowerCase()) !== NON_EXIST_INDEX;
    });
});

/**
 * Returns stored selected bucket names.
 */
const selectedBucketNames = computed((): string[] => {
    return agStore.state.selectedBucketNames;
});

/**
 * Clears selection of specific buckets and closes dropdown.
 */
function clearSelectedBuckets(): void {
    agStore.clearSelection();
    closeDropdown();
}

/**
 * Toggles bucket selection.
 */
function toggleBucketSelection(name: string): void {
    agStore.toggleBucketSelection(name);
}

/**
 * Indicates if bucket name is selected.
 * @param name
 */
function isNameSelected(name: string): boolean {
    return selectedBucketNames.value.includes(name);
}

/**
 * Closes dropdown.
 */
function closeDropdown(): void {
    appStore.closeDropdowns();
}
</script>

<style scoped lang="scss">
    ::-webkit-scrollbar,
    ::-webkit-scrollbar-track,
    ::-webkit-scrollbar-thumb {
        margin: 0;
        width: 0;
    }

    .buckets-dropdown {
        position: absolute;
        z-index: 2;
        left: 0;
        top: calc(100% + 5px);
        box-shadow: 0 20px 34px rgb(10 27 44 / 28%);
        border-radius: 6px;
        background-color: #fff;
        border: 1px solid rgb(56 75 101 / 40%);
        width: 100%;
        padding: 10px 0;
        cursor: default;

        &__container {
            overflow: hidden auto;
            width: 100%;
            max-height: 230px;
            background-color: #fff;
            border-radius: 6px;
            font-family: 'font_regular', sans-serif;
            font-style: normal;
            font-weight: normal;
            font-size: 16px;
            line-height: 21px;
            color: #384b65;

            &__search {
                padding: 5px 10px;
                width: calc(100% - 20px);

                &__input {
                    font-size: 14px;
                    line-height: 18px;
                    border-radius: 6px;
                    width: calc(100% - 30px);
                    padding: 5px;
                }
            }

            &__all {
                margin: 0;
                cursor: pointer;
                background-color: #fff;
                width: calc(100% - 50px);
                padding: 15px 0 15px 50px;

                &:hover {
                    background-color: #ecedf2;
                }
            }

            &__no-buckets {
                font-family: 'font_bold', sans-serif;
                margin: 0;
                font-size: 18px;
                line-height: 24px;
                cursor: default;
                color: #000;
                background-color: #fff;
                width: 100%;
                padding: 15px 0;
                text-align: center;
            }

            &__choices {

                &__item__unselect-icon {
                    opacity: 0;
                }

                .selected {
                    background-color: #f5f6fa;

                    &:hover {

                        .buckets-dropdown__container__choices__item__unselect-icon {
                            opacity: 1 !important;
                        }
                    }
                }

                &__item {
                    display: flex;
                    align-items: center;
                    justify-content: space-between;
                    padding: 15px 20px;
                    width: calc(100% - 40px);
                    cursor: pointer;

                    &__left {
                        display: flex;
                        align-items: center;
                        max-width: 100%;

                        &__label {
                            margin: 0 0 0 15px;
                            text-overflow: ellipsis;
                            white-space: nowrap;
                            overflow: hidden;
                        }
                    }

                    &:hover {
                        background-color: #ecedf2;
                    }
                }
            }
        }
    }

    .check-icon {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 14px;
        height: 11px;
        max-width: 14px;
        max-height: 11px;
    }
</style>

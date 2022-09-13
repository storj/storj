// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-click-outside="closeDropdown" :class="`buckets-dropdown ${showScrollbar ? 'show-scroll' : ''}`">
        <div :class="`buckets-dropdown__container ${showScrollbar ? 'show-scroll' : ''}`">
            <p class="buckets-dropdown__container__all" @click.stop="selectAllBuckets">
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

<script lang="ts">
import { Component, Vue, Prop } from 'vue-property-decorator';

import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

import SelectionIcon from '@/../static/images/accessGrants/selection.svg';
import UnselectIcon from '@/../static/images/accessGrants/unselect.svg';

// @vue/component
@Component({
    components: {
        SelectionIcon,
        UnselectIcon,
    },
})
export default class BucketsDropdown extends Vue {
    @Prop({ default: false })
    private readonly showScrollbar: boolean;
    public bucketSearch = '';

    /**
     * Clears selection of specific buckets and closes dropdown.
     */
    public selectAllBuckets(): void {
        this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CLEAR_SELECTION);
        this.closeDropdown();
    }

    /**
     * Toggles bucket selection.
     */
    public toggleBucketSelection(name: string): void {
        this.$store.dispatch(ACCESS_GRANTS_ACTIONS.TOGGLE_BUCKET_SELECTION, name);
    }

    /**
     * Indicates if bucket name is selected.
     * @param name
     */
    public isNameSelected(name: string): boolean {
        return this.selectedBucketNames.includes(name);
    }

    /**
     * Closes dropdown.
     */
    public closeDropdown(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);
    }

    /**
     * Returns stored bucket names list filtered by search string.
     */
    public get bucketsList(): string[] {
        const NON_EXIST_INDEX = -1;
        const buckets: string[] = this.$store.state.bucketUsageModule.allBucketNames;

        return buckets.filter((name: string) => {
            return name.indexOf(this.bucketSearch.toLowerCase()) !== NON_EXIST_INDEX;
        });
    }

    /**
     * Returns stored selected bucket names.
     */
    public get selectedBucketNames(): string[] {
        return this.$store.state.accessGrantsModule.selectedBucketNames;
    }
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
            overflow-y: auto;
            overflow-x: hidden;
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

    .show-scroll {
        padding-right: 2px;
        width: calc(100% - 2px);
    }

    .show-scroll::-webkit-scrollbar {
        width: 4px;
    }
</style>

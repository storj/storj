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
        <th slot="options" v-click-outside="closeDropdown" :class="{active: isDropdownOpen}" class="bucket-item__functional options overflow-visible" @click.stop="openDropdown(dropdownKey)">
            <dots-icon />
            <div v-if="isDropdownOpen" class="bucket-item__functional__dropdown">
                <div class="bucket-item__functional__dropdown__item" @click.stop="onDetailsClick">
                    <details-icon />
                    <p class="bucket-item__functional__dropdown__item__label">View Bucket Details</p>
                </div>
                <div class="bucket-item__functional__dropdown__item delete" @click.stop="onDeleteClick">
                    <delete-icon />
                    <p class="bucket-item__functional__dropdown__item__label">Delete Bucket</p>
                </div>
            </div>
        </th>
    </table-item>
</template>

<script lang="ts">
import { Component, Prop } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { Bucket } from '@/types/buckets';
import { LocalData } from '@/utils/localData';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';

import TableItem from '@/components/common/TableItem.vue';
import Resizable from '@/components/common/Resizable.vue';

import DeleteIcon from '@/../static/images/objects/delete.svg';
import DetailsIcon from '@/../static/images/objects/details.svg';
import DotsIcon from '@/../static/images/objects/dots.svg';

// @vue/component
@Component({
    components: {
        TableItem,
        DotsIcon,
        DeleteIcon,
        DetailsIcon,
    },
})
export default class BucketItem extends Resizable {
    @Prop({ default: null })
    public readonly itemData: Bucket;
    @Prop({ default: () => () => {} })
    public readonly openDropdown;
    @Prop({ default: () => (_: string) => {} })
    public readonly onClick: (bucket: string) => void;
    @Prop({ default: false })
    public readonly isDropdownOpen: boolean;
    @Prop({ default: true })
    public readonly showGuide: boolean;
    @Prop({ default: -1 })
    public readonly dropdownKey: number;

    public isGuideShown = true;

    public mounted(): void {
        this.isGuideShown = !LocalData.getBucketGuideHidden();
    }

    public get shouldShowGuide(): boolean {
        return this.showGuide && this.isGuideShown;
    }

    /**
     * Returns formatted date.
     */
    public get formattedDate(): string {
        return this.itemData.since.toLocaleString('en-US', { day: '2-digit', month: 'numeric', year: 'numeric' }) || '';
    }

    public get itemToRender(): { [key: string]: string | string[] } {
        if (this.screenWidth > 875) return {
            name: this.itemData.name,
            storage: `${this.itemData.storage.toFixed(2)}GB`,
            bandwidth: `${this.itemData.egress.toFixed(2)}GB`,
            objects: this.itemData.objectCount.toString(),
            segments: this.itemData.segmentCount.toString(),
            date: this.formattedDate,
        };

        return { info: [
            this.itemData.name,
            `Storage ${this.itemData.storage.toFixed(2)}GB`,
            `Bandwidth ${this.itemData.egress.toFixed(2)}GB`,
            `Objects ${this.itemData.objectCount.toString()}`,
            `Segments ${this.itemData.segmentCount.toString()}`,
            `Created ${this.formattedDate}`,
        ] };
    }

    /*
    * Permanently hide the upload guide
    * */
    public hideGuidePermanently(): void {
        this.isGuideShown = false;
        LocalData.setBucketGuideHidden();
    }

    /**
     * Closes dropdown.
     */
    public closeDropdown(): void {
        this.openDropdown(-1);
    }

    /**
     * Holds on delete click logic.
     */
    public onDeleteClick(): void {
        this.$store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.deleteBucket);
        this.closeDropdown();
    }

    /**
     * Redirects to bucket details page.
     */
    public onDetailsClick(): void {
        this.$router.push({
            name: RouteConfig.Buckets.with(RouteConfig.BucketsDetails).name,
            params: {
                bucketName: this.itemData.name,
                backRoute: this.$route.name || '',
            },
        });

        this.closeDropdown();
    }
}
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

                @media screen and (max-width: 320px) {
                    transform: translateX(-80%);
                }

                @media screen and (max-width: 375px) and (min-width: 350px) {
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

                    @media screen and (max-width: 550px) {
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

    @media screen and (max-width: 1400px) {

        :deep(th) {
            max-width: 25rem;
        }
    }

    @media screen and (max-width: 1100px) {

        :deep(th) {
            max-width: 20rem;
        }
    }

    @media screen and (max-width: 1000px) {

        :deep(th) {
            max-width: 15rem;
        }
    }

    @media screen and (max-width: 940px) {

        :deep(th) {
            max-width: 10rem;
        }
    }
</style>

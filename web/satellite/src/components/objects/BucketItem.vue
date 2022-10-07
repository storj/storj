// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table-item
        :item="itemToRender"
        :on-click="onClick"
        :show-guide="shouldShowGuide"
        :hide-guide="hideGuidePermanently"
        table-type="bucket"
    >
        <th slot="options" v-click-outside="closeDropdown" class="bucket-item__functional options overflow-visible" @click.stop="openDropdown(dropdownKey)">
            <dots-icon />
            <div v-if="isDropdownOpen" class="bucket-item__functional__dropdown">
                <div class="bucket-item__functional__dropdown__item" @click.stop="onDetailsClick">
                    <details-icon />
                    <p class="bucket-item__functional__dropdown__item__label">View Bucket Details</p>
                </div>
                <div class="bucket-item__functional__dropdown__item" @click.stop="onDeleteClick">
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
    public readonly showDeleteBucketPopup: () => void;
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
        return this.itemData.since.toLocaleString() || '';
    }

    public get itemToRender(): { [key: string]: string | string[] } {
        if (!this.isMobile) return { name: this.itemData.name, date: this.formattedDate };

        return { info: [ this.itemData.name, `Created ${this.formattedDate}` ] };
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
        this.showDeleteBucketPopup();
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

            &__dropdown {
                position: absolute;
                top: 25px;
                right: 15px;
                background: #fff;
                box-shadow: 0 20px 34px rgb(10 27 44 / 28%);
                border-radius: 6px;
                width: 255px;
                padding: 10px 0;
                z-index: 100;

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

                        svg :deep(path) {
                            fill: #0068dc;
                            stroke: #0068dc;
                        }
                    }
                }
            }
        }

        &:hover {
            background-color: #e6e9ef;
        }
    }
</style>

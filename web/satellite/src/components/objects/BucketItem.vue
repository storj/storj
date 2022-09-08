// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table-item
        :item="itemToRender"
        :on-click="onClick"
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
            <div v-if="shouldShowGuide" class="bucket-item__functional__message" @click.stop>
                <p class="bucket-item__functional__message__title">Upload</p>
                <p class="bucket-item__functional__message__content">To upload files, open an existing bucket or create a new one.</p>
                <div class="bucket-item__functional__message__actions">
                    <div class="bucket-item__functional__message__actions__primary" @click.stop="hideGuidePermanently">
                        <p class="bucket-item__functional__message__actions__primary__label">OK</p>
                    </div>
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
            position: relative;
            cursor: pointer;

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

            &__message {
                position: absolute;
                top: 80%;
                width: 25rem;
                display: flex;
                flex-direction: column;
                align-items: start;
                transform: translateX(-100%);
                background-color: #0149ff;
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
                    border-color: #0149ff transparent transparent;
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
                    justify-content: end;
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
                            color: #0149ff;
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

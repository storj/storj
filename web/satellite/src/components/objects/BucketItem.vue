// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="bucket-item">
        <div class="bucket-item__name">
            <BucketIcon />
            <p class="bucket-item__name__value">{{ itemData.Name }}</p>
        </div>
        <p class="bucket-item__date">{{ formattedDate }}</p>
        <div v-click-outside="closeDropdown" class="bucket-item__functional" @click.stop="openDropdown(dropdownKey)">
            <DotsIcon />
            <div v-if="isDropdownOpen" class="bucket-item__functional__dropdown">
                <div class="bucket-item__functional__dropdown__item" @click.stop="onDeleteClick">
                    <DeleteIcon />
                    <p class="bucket-item__functional__dropdown__item__label">Delete</p>
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Bucket } from '@aws-sdk/client-s3';
import { Component, Prop, Vue } from 'vue-property-decorator';

import BucketIcon from '@/../static/images/objects/bucketItem.svg';
import DeleteIcon from '@/../static/images/objects/delete.svg';
import DotsIcon from '@/../static/images/objects/dots.svg';

// @vue/component
@Component({
    components: {
        BucketIcon,
        DotsIcon,
        DeleteIcon,
    },
})
export default class BucketItem extends Vue {
    @Prop({ default: null })
    public readonly itemData: Bucket;
    @Prop({ default: () => () => {} })
    public readonly showDeleteBucketPopup: () => void;
    @Prop({ default: () => () => {} })
    public readonly openDropdown;
    @Prop({ default: false })
    public readonly isDropdownOpen: boolean;
    @Prop({ default: -1 })
    public readonly dropdownKey: number;

    public isRequestProcessing = false;
    public errorMessage = '';

    /**
     * Returns formatted date.
     */
    public get formattedDate(): string | undefined {
        return this.itemData.CreationDate?.toLocaleString();
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
}
</script>

<style scoped lang="scss">
    .bucket-item {
        font-family: 'font_regular', sans-serif;
        font-style: normal;
        display: flex;
        align-items: center;
        padding: 25px 20px;
        width: calc(100% - 40px);
        font-weight: normal;
        font-size: 14px;
        line-height: 19px;
        color: #1b2533;
        cursor: pointer;

        &__name {
            display: flex;
            align-items: center;
            width: 70%;

            &__value {
                margin: 0 0 0 17px;
            }
        }

        &__date {
            width: 30%;
            margin: 0;
        }

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

                        .bucket-delete-path {
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

// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="bucket-settings-nav" @click.stop.prevent="isDropdownOpen = !isDropdownOpen">
        <div class="bucket-settings-nav__button">
            <GearIcon />
            <arrow-down-icon />
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

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';

import ArrowDownIcon from '@/../static/images/objects/arrowDown.svg';
import DetailsIcon from '@/../static/images/objects/details.svg';
import ShareIcon from '@/../static/images/objects/share.svg';
import GearIcon from '@/../static/images/common/gearIcon.svg';

// @vue/component
@Component({
    components: {
        ArrowDownIcon,
        GearIcon,
        DetailsIcon,
        ShareIcon,
    },
})
export default class BucketSettingsNav extends Vue {
    @Prop({ default: '' })
    public readonly bucketName: string;

    public isDropdownOpen = false;

    public closeDropdown(): void {
        if (!this.isDropdownOpen) return;

        this.isDropdownOpen = false;
    }

    /**
     * Redirects to bucket details page.
     */
    public onDetailsClick(): void {
        this.$router.push({
            name: RouteConfig.BucketsDetails.name,
            params: {
                bucketName: this.bucketName,
                backRoute: this.$route.name ? this.$route.name : '',
            },
        });
        this.isDropdownOpen = false;
    }

    /**
     * Toggles share bucket modal.
     */
    public onShareBucketClick(): void {
        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_SHARE_BUCKET_MODAL_SHOWN);
        this.isDropdownOpen = false;
    }

    /**
     * Returns files amount from store.
     */
    public get filesCount(): number {
        return this.$store.getters['files/sortedFiles'].length;
    }
}
</script>

<style scoped lang="scss">
.bucket-settings-nav {
    position: relative;
    font-family: 'font_regular', sans-serif;
    font-style: normal;
    display: flex;
    align-items: center;
    padding: 14px 16px;
    height: 44px;
    box-sizing: border-box;
    font-weight: normal;
    font-size: 14px;
    line-height: 19px;
    color: #1b2533;
    cursor: pointer;
    background: white;
    border: 1px solid #d8dee3;
    border-radius: 8px;

    &__button {
        display: flex;
        align-items: center;
        justify-content: space-between;
        cursor: pointer;

        svg:first-of-type {
            margin-right: 10px;
        }
    }

    &__dropdown {
        position: absolute;
        top: 50px;
        right: 0;
        display: flex;
        flex-direction: column;
        justify-content: center;
        padding: 10px 0;
        background: #fff;
        box-shadow: 0 20px 34px rgb(10 27 44 / 28%);
        border-radius: 6px;
        width: 255px;
        z-index: 100;

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
                fill: #0068dc;
            }
        }
    }
}
</style>

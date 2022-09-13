// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="container">
        <div class="container__header" :title="itemData.name">
            <BucketIcon class="container__header__icon" />
            <p class="container__header__name">{{ itemData.name }}</p>
        </div>
        <div class="container__item">
            <div class="container__item__inner">
                <p class="container__item__inner__label">Storage</p>
                <p class="container__item__inner__value">{{ itemData.storage.toFixed(2) }}GB</p>
            </div>
            <div class="container__item__inner">
                <p class="container__item__inner__label">Bandwidth</p>
                <p class="container__item__inner__value">{{ itemData.egress.toFixed(2) }}GB</p>
            </div>
        </div>
        <div class="container__item">
            <div class="container__item__inner">
                <p class="container__item__inner__label">Objects</p>
                <p class="container__item__inner__value">{{ itemData.objectCount.toString() }}</p>
            </div>
            <div class="container__item__inner">
                <p class="container__item__inner__label">Segments</p>
                <p class="container__item__inner__value">{{ itemData.segmentCount.toString() }}</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { Bucket } from '@/types/buckets';

import BucketIcon from '@/../static/images/project/bucket.svg';

// @vue/component
@Component({
    components: {
        BucketIcon,
    },
})
export default class BucketItem extends Vue {
    @Prop({ default: () => new Bucket('', 0, 0, 0, 0, new Date(), new Date()) })
    private readonly itemData: Bucket;
}
</script>

<style scoped lang="scss">
    .container {
        padding: 20px 40px;
        outline: none;
        display: flex;
        margin-bottom: 1px;
        box-sizing: border-box;
        font-family: 'font_medium', sans-serif;
        font-size: 16px;

        &__header {
            width: 100%;
            display: flex;
            align-items: center;

            &__icon {
                display: none;
                margin-right: 10px;
            }

            &__name {
                white-space: nowrap;
                overflow: hidden;
                text-overflow: ellipsis;
            }
        }

        &__item {
            display: flex;
            width: 100%;

            &__inner {
                width: 50%;

                &__label {
                    display: none;
                    margin-bottom: 4px;
                    font-family: 'font_regular', sans-serif;
                    font-size: 14px;
                }
            }
        }
    }

    @media screen and (max-width: 960px) {

        .container {
            flex-wrap: wrap;
            padding: 20px 24px;

            &__header {

                &__icon {
                    display: block;
                }

                &__name {
                    font-family: 'font_bold', sans-serif;
                }
            }

            &__item {
                width: 50%;
                margin-top: 16px;

                &__inner__label {
                    display: block;
                }
            }
        }
    }

    @media screen and (max-width: 600px) {

        .container {
            flex-direction: column;

            &__item {
                width: 100%;
            }
        }
    }
</style>

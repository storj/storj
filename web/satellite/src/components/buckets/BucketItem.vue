// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="container">
        <div class="container__item" :title="itemData.bucketName">{{ itemData.formattedBucketName() }}</div>
        <div class="container__item">{{ itemData.storage.toFixed(4) }}</div>
        <div class="container__item">{{ itemData.egress.toFixed(4) }}</div>
        <div class="container__item">{{ itemData.objectCount.toString() }}</div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { Bucket } from '@/types/buckets';

@Component
export default class BucketItem extends Vue {
    @Prop({default: () => new Bucket('', 0, 0, 0, new Date(), new Date())})
    private readonly itemData: Bucket;
}
</script>

<style scoped lang="scss">
    .container {
        padding: 25px 0;
        -webkit-user-select: none;
        -moz-user-select: none;
        -ms-user-select: none;
        user-select: none;
        outline: none;
        display: flex;
        background: #fff;
        margin-bottom: 1px;

        &__item {
            width: 25%;
            padding-left: 26px;
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            margin: 0;
        }
    }
</style>

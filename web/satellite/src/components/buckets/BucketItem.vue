// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="container">
        <div class="container__item">{{ name }}</div>
        <div class="container__item">{{ storage }}</div>
        <div class="container__item">{{ egress }}</div>
        <div class="container__item">{{ objectCount }}</div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { Bucket } from '@/types/buckets';

// TODO: should it be functional?
@Component
export default class BucketItem extends Vue {
    @Prop({default: () => new Bucket('', 0, 0, 0, new Date(), new Date())})
    private readonly itemData: Bucket;

    public get name(): string {
        return this.itemData.formattedBucketName();
    }

    public get storage(): string {
        return this.itemData.storage.toFixed(4);
    }

    public get egress(): string {
        return this.itemData.egress.toFixed(4);
    }

    public get objectCount(): string {
        return this.itemData.objectCount.toString();
    }
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

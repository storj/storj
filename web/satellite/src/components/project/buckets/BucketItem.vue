// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="container">
        <p class="container__item name" :title="itemData.name">{{ itemData.name }}</p>
        <p class="container__item">{{ itemData.storage.toFixed(2) }}GB</p>
        <p class="container__item">{{ itemData.egress.toFixed(2) }}GB</p>
        <p class="container__item">{{ itemData.objectCount.toString() }}</p>
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
        padding: 20px 40px;
        outline: none;
        display: flex;
        background: #fff;
        margin-bottom: 1px;
        width: calc(100% - 80px);

        &__item {
            width: 20%;
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            margin: 0;
        }
    }

    .name {
        width: 40%;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }
</style>

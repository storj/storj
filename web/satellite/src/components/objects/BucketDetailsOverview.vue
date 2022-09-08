// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table class="bucket-details-overview" border="0" cellpadding="0" cellspacing="0">
        <tr v-for="item in tableData" :key="item.label" class="bucket-details-overview__item">
            <th class="align-left bold title-row">{{ item.label }}</th>
            <th class="align-left">{{ item.value }}</th>
        </tr>
    </table>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { Bucket } from '@/types/buckets';

type TableData = { label: string, value: string }[];

// @vue/component
@Component
export default class BucketDetailsOverview extends Vue {
    @Prop({ default: null })
    public readonly bucket: Bucket;

    public get tableData(): TableData {
        return [
            { label: 'Name', value: this.bucket.name },
            { label: 'Date Created', value: this.bucket.since.toUTCString() },
            { label: 'Last Updated', value: this.bucket.before.toUTCString() },
            { label: 'Object Count', value: `${this.bucket.objectCount}` },
        ];
    }
}
</script>

<style lang="scss" scoped>
    .bucket-details-overview {
        color: #56606d;
        font-family: 'font_regular', sans-serif;
        font-size: 14px;
        line-height: 20px;
        padding: 10px 0;
        background: #fff;
        border-radius: 8px;
        width: 100%;
        border: 1px solid #dadfe7;

        &__item {
            height: 56px;
            text-align: right;

            th {
                box-sizing: border-box;
                padding: 0 32px;
                min-width: 140px;
                white-space: nowrap;
                text-overflow: ellipsis;
                position: relative;
                overflow: hidden;
            }

            &:nth-of-type(odd) {
                background: #fafafa;
            }
        }
    }

    .title-row {
        width: 20%;
    }

    .align-left {
        text-align: left;
    }

    .bold {
        font-family: 'font_bold', sans-serif;
    }
</style>

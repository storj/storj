// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <tr
        class="item-container"
        :class="{ 'selected': selected }"
        @click="onClick"
    >
        <th v-if="selectable" class="icon select">
            <v-table-checkbox :disabled="selectDisabled" :value="selected" @checkChange="onChange" />
        </th>
        <th
            v-for="(val, key, index) in item" :key="index" class="align-left data"
            :class="{'guide-container': showBucketGuide(index)}"
        >
            <BucketGuide v-if="showBucketGuide(index)" :hide-guide="hideGuide" />
            <div v-if="Array.isArray(val)" class="few-items">
                <p v-for="str in val" :key="str" class="array-val">{{ str }}</p>
            </div>
            <div v-else class="table-item">
                <BucketIcon v-if="(tableType.toLowerCase() === 'bucket') && (index == 0)" class="item-icon" />
                <p :class="{primary: index === 0}">{{ val }}</p>
                <div v-if="showBucketGuide(index)" class="animation"><span><span /></span></div>
            </div>
        </th>
        <slot name="options" />
    </tr>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VTableCheckbox from '@/components/common/VTableCheckbox.vue';
import BucketGuide from '@/components/objects/BucketGuide.vue';

import BucketIcon from '@/../static/images/objects/bucketIcon.svg';

// @vue/component
@Component({
    components: {
        VTableCheckbox,
        BucketGuide,
        BucketIcon,
    },
})
export default class TableItem extends Vue {
    @Prop({ default: false })
    public readonly selectDisabled: boolean;
    @Prop({ default: false })
    public readonly selected: boolean;
    @Prop({ default: false })
    public readonly selectable: boolean;
    @Prop({ default: false })
    public readonly showGuide: boolean;
    @Prop({ default: 'none' })
    private readonly tableType: string;
    @Prop({ default: () => {} })
    public readonly item: object;
    @Prop({ default: null })
    public readonly onClick: (data?: unknown) => void;
    @Prop({ default: null })
    public readonly hideGuide: () => void;

    public onChange(value: boolean): void {
        this.$emit('selectChange', value);
    }

    public showBucketGuide(index: number): boolean {
        return (this.tableType.toLowerCase() === 'bucket') && (index === 0) && this.showGuide;
    }
}
</script>

<style scoped lang="scss">
    @mixin keyframes() {
        @keyframes pulse {

            0% {
                opacity: 0.75;
                transform: scale(1);
            }

            25% {
                opacity: 0.75;
                transform: scale(1);
            }

            100% {
                opacity: 0;
                transform: scale(2.5);
            }
        }
    }

    @include keyframes;

    .animation {
        border-radius: 50%;
        height: 8px;
        width: 8px;
        margin-left: 23px;
        margin-top: 5px;
        background-color: #0149ff;

        > span {
            animation: pulse 1s linear infinite;
            border-radius: 50%;
            display: block;
            height: 8px;
            width: 8px;

            > span {
                animation: pulse 1s linear infinite;
                border-radius: 50%;
                display: block;
                height: 8px;
                width: 8px;
            }
        }

        span {
            background-color: rgb(1 73 255 / 2000%);

            &:after {
                background-color: rgb(1 73 255 / 2000%);
            }
        }
    }

    tr {
        cursor: pointer;

        &:hover .table-item .primary {
            color: #0149ff;
        }

        &.selected {
            background: #f0f3f8;
        }
    }

    .few-items {
        display: flex;
        flex-direction: column;
        justify-content: space-between;
    }

    .array-val {
        font-family: 'font_regular', sans-serif;
        font-size: 0.75rem;
        line-height: 1.25rem;

        &:first-of-type {
            font-family: 'font_bold', sans-serif;
            font-size: 0.875rem;
            margin-bottom: 3px;
        }
    }

    .table-item {
        display: flex;
    }

    .item-container {
        position: relative;
    }

    .item-icon {
        margin-right: 12px;
    }

    .guide-container {
        position: relative;
        overflow: visible;
    }
</style>

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
            v-for="(val, _, index) in item" :key="index" class="align-left data"
            :class="{'overflow-visible': showBucketGuide(index)}"
        >
            <div v-if="Array.isArray(val)" class="few-items">
                <p v-for="str in val" :key="str" class="array-val">{{ str }}</p>
            </div>
            <div v-else class="table-item">
                <BucketIcon v-if="(tableType.toLowerCase() === 'bucket') && (index === 0)" class="item-icon" />
                <FileIcon v-else-if="(tableType.toLowerCase() === 'file') && (index === 0)" class="item-icon" />
                <FolderIcon v-else-if="(tableType.toLowerCase() === 'folder') && (index === 0)" class="item-icon" />
                <p :class="{primary: index === 0}" @click.stop="(e) => cellContentClicked(index, e)">
                    <middle-truncate v-if="(tableType.toLowerCase() === 'file')" :text="val" />
                    <span v-else>{{ val }}</span>
                </p>
                <div v-if="showBucketGuide(index)" class="animation">
                    <span><span /></span>
                    <BucketGuide :hide-guide="hideGuide" />
                </div>
            </div>
        </th>
        <slot name="options" />
    </tr>
</template>

<script setup lang="ts">
import VTableCheckbox from '@/components/common/VTableCheckbox.vue';
import BucketGuide from '@/components/objects/BucketGuide.vue';
import MiddleTruncate from '@/components/browser/MiddleTruncate.vue';

import FolderIcon from '@/../static/images/objects/folder.svg';
import BucketIcon from '@/../static/images/objects/bucketIcon.svg';
import FileIcon from '@/../static/images/objects/file.svg';

const props = withDefaults(defineProps<{
    selectDisabled?: boolean;
    selected?: boolean;
    selectable?: boolean;
    showGuide?: boolean;
    tableType?: string;
    item?: object;
    onClick?: (data?: unknown) => void;
    // event for the first cell of this item.
    hideGuide?: () => void;
    onPrimaryClick: (data?: unknown) => void;
}>(), {
    selectDisabled: false,
    selected: false,
    selectable: false,
    showGuide: false,
    tableType: 'none',
    item: () => ({}),
    onClick: () => {},
    hideGuide: () => {},
});

const emit = defineEmits(['selectChange']);

function onChange(value: boolean): void {
    emit('selectChange', value);
}

function showBucketGuide(index: number): boolean {
    return (props.tableType.toLowerCase() === 'bucket') && (index === 0) && props.showGuide;
}

function cellContentClicked(cellIndex: number, event: Event) {
    if (cellIndex === 0 && props.onPrimaryClick) {
        props.onPrimaryClick(event);
        return;
    }
    // trigger default item onClick instead.
    props.onClick();
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
        position: relative;

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

        &:hover .table-item {

            svg :deep(path) {
                fill: var(--c-blue-3);
            }
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
        align-items: center;
    }

    .item-container {
        position: relative;
    }

    .item-icon {
        margin-right: 12px;
        min-width: 18px;
    }
</style>

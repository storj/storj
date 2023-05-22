// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <tr
        class="item-container"
        :class="{ 'selected': selected }"
        @click="onClick"
    >
        <th v-if="selectable" class="icon select" @click.stop="selectClicked">
            <v-table-checkbox v-if="!selectHidden" :disabled="selectDisabled || selectHidden" :value="selected" @selectClicked="selectClicked" />
        </th>
        <th
            v-for="(val, keyVal, index) in item" :key="index" class="align-left data"
            :class="{'overflow-visible': showBucketGuide(index)}"
        >
            <div v-if="Array.isArray(val)" class="few-items">
                <p v-for="str in val" :key="str" class="array-val">{{ str }}</p>
            </div>
            <div v-else class="table-item">
                <div v-if="icon && index === 0" class="item-icon file-background">
                    <component :is="icon" />
                </div>
                <p :class="{primary: index === 0}" :title="val" @click.stop="(e) => cellContentClicked(index, e)">
                    <middle-truncate v-if="keyVal === 'fileName'" :text="val" />
                    <project-ownership-tag v-else-if="keyVal === 'owner'" no-icon :is-owner="val" />
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
import { computed, Component } from 'vue';

import VTableCheckbox from '@/components/common/VTableCheckbox.vue';
import BucketGuide from '@/components/objects/BucketGuide.vue';
import MiddleTruncate from '@/components/browser/MiddleTruncate.vue';
import ProjectOwnershipTag from '@/components/project/ProjectOwnershipTag.vue';

import TableLockedIcon from '@/../static/images/browser/tableLocked.svg';
import ColorFolderIcon from '@/../static/images/objects/colorFolder.svg';
import ColorBucketIcon from '@/../static/images/objects/colorBucket.svg';
import FileIcon from '@/../static/images/objects/file.svg';
import AudioIcon from '@/../static/images/objects/audio.svg';
import VideoIcon from '@/../static/images/objects/video.svg';
import ChevronLeftIcon from '@/../static/images/objects/chevronLeft.svg';
import GraphIcon from '@/../static/images/objects/graph.svg';
import PdfIcon from '@/../static/images/objects/pdf.svg';
import PictureIcon from '@/../static/images/objects/picture.svg';
import TxtIcon from '@/../static/images/objects/txt.svg';
import ZipIcon from '@/../static/images/objects/zip.svg';

const props = withDefaults(defineProps<{
    selectDisabled?: boolean;
    selectHidden?: boolean;
    selected?: boolean;
    selectable?: boolean;
    showGuide?: boolean;
    itemType?: string;
    item?: object;
    onClick?: (data?: unknown) => void;
    hideGuide?: () => void;
    // event for the first cell of this item.
    onPrimaryClick?: (data?: unknown) => void;
}>(), {
    selectDisabled: false,
    selectHidden: false,
    selected: false,
    selectable: false,
    showGuide: false,
    itemType: 'none',
    item: () => ({}),
    onClick: () => {},
    hideGuide: () => {},
    onPrimaryClick: undefined,
});

const emit = defineEmits(['selectClicked']);

const icons = new Map<string, Component>([
    ['locked', TableLockedIcon],
    ['bucket', ColorBucketIcon],
    ['folder', ColorFolderIcon],
    ['file', FileIcon],
    ['audio', AudioIcon],
    ['video', VideoIcon],
    ['back', ChevronLeftIcon],
    ['spreadsheet', GraphIcon],
    ['pdf', PdfIcon],
    ['image', PictureIcon],
    ['text', TxtIcon],
    ['archive', ZipIcon],
]);

const icon = computed(() => icons.get(props.itemType.toLowerCase()));

function selectClicked(event: Event): void {
    emit('selectClicked', event);
}

function showBucketGuide(index: number): boolean {
    return (props.itemType?.toLowerCase() === 'bucket') && (index === 0) && props.showGuide;
}

function cellContentClicked(cellIndex: number, event: Event) {
    if (cellIndex === 0 && props.onPrimaryClick) {
        props.onPrimaryClick(event);
        return;
    }
    // trigger default item onClick instead.
    if (props.onClick) {
        props.onClick();
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

        &:hover {
            background: var(--c-grey-1);

            .table-item {

                .primary {
                    color: var(--c-blue-3);
                }
            }
        }

        &.selected {
            background: var(--c-yellow-1);

            :deep(.select) {
                background: var(--c-yellow-1);
            }
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

    .file-background {
        background: var(--c-white);
        border: 1px solid var(--c-grey-2);
        padding: 2px;
        border-radius: 8px;
        height: 32px;
        min-width: 32px;
        display: flex;
        align-items: center;
        justify-content: center;
    }
</style>

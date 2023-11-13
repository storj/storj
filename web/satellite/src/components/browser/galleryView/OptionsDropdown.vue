// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="options">
        <div class="options__item" @click="onDistribution">
            <MapIcon />
            <p class="options__item__label">Distribution</p>
        </div>
        <div class="options__item" @click="onViewDetails">
            <DetailsIcon />
            <p class="options__item__label">View details</p>
        </div>
        <div class="options__item" @click="onDownload">
            <DownloadIcon />
            <p class="options__item__label">Download</p>
        </div>
        <div class="options__item" @click="onShare">
            <ShareIcon />
            <p class="options__item__label">Share</p>
        </div>
        <div class="options__item" @click="onDelete">
            <DeleteIcon />
            <p class="options__item__label">Delete</p>
        </div>
    </div>
</template>

<script setup lang="ts">
import DetailsIcon from '@/../static/images/browser/galleryView/details.svg';
import DownloadIcon from '@/../static/images/browser/galleryView/download.svg';
import ShareIcon from '@/../static/images/browser/galleryView/share.svg';
import DeleteIcon from '@/../static/images/browser/galleryView/delete.svg';
import MapIcon from '@/../static/images/browser/galleryView/map.svg';

const props = defineProps<{
    onDistribution: () => void
    onViewDetails: () => void
    onDownload: () => void
    onShare: () => void
    onDelete: () => void
}>();
</script>

<style scoped lang="scss">
.options {
    position: absolute;
    left: 0;
    top: calc(100% + 10px);
    width: 240px;
    border: 1px solid var(--c-grey-2);
    filter: drop-shadow(0 7px 20px rgb(0 0 0 / 15%));
    border-radius: 8px;
    background-color: var(--c-white);
    font-family: 'font_regular', sans-serif;
    z-index: 1;

    @media screen and (width < 1100px) {
        width: 155px;
    }

    &__item {
        display: flex;
        align-items: center;
        cursor: pointer;
        padding: 16px;

        svg {
            width: 18px;
            height: 18px;

            :deep(path) {
                fill: var(--c-grey-6);
            }
        }

        &__label {
            margin-left: 16px;
            font-size: 14px;
            line-height: 20px;
            color: var(--c-grey-6);
        }

        &:first-of-type {
            border-radius: 8px 8px 0 0;
        }

        &:last-of-type {
            border-radius: 0 0 8px 8px;
        }

        &:hover {
            background-color: var(--c-grey-0);
        }
    }
}
</style>

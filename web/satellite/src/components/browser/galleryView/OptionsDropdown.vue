// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="options">
        <div class="options__item" @click="onViewDetails">
            <DetailsIcon />
            <p class="options__item__label">View details</p>
        </div>
        <div class="options__item" @click="onDownload">
            <SmallDownloadIcon />
            <p class="options__item__label">Download</p>
        </div>
        <div class="options__item" @click="onShare">
            <SmallShareIcon />
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
import SmallDownloadIcon from '@/../static/images/browser/galleryView/downloadSmall.svg';
import SmallShareIcon from '@/../static/images/browser/galleryView/shareSmall.svg';
import DeleteIcon from '@/../static/images/browser/galleryView/delete.svg';

const props = defineProps<{
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

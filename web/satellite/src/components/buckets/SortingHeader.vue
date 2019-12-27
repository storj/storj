// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="sort-header-container">
        <div class="sort-header-container__item">
            <p class="sort-header-container__item__name">Bucket Name</p>
        </div>
        <div class="sort-header-container__item">
            <p class="sort-header-container__item__name">Storage Used, GB</p>
        </div>
        <div class="sort-header-container__item">
            <p class="sort-header-container__item__name">Egress Used, GB</p>
        </div>
        <div class="sort-header-container__item">
            <p class="sort-header-container__item__name">Objects Stored</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

@Component
export default class SortBucketsHeader extends Vue {}
</script>

<style scoped lang="scss">
    .sort-header-container {
        display: flex;
        background: #fff;
        opacity: 0.5;
        max-height: 40px;
        align-items: center;
        user-select: none;

        &__item {
            width: 25%;
            margin: 0;
            text-align: left;
            padding-left: 26px;

            &__name {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 23px;
                color: #2a2a32;
            }
        }
    }
</style>

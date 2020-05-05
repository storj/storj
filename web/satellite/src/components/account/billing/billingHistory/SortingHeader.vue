// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="sort-header-container">
        <div class="sort-header-container__item date">
            <p class="sort-header-container__item__name">Date</p>
        </div>
        <div class="sort-header-container__item description">
            <p class="sort-header-container__item__name">Description</p>
        </div>
        <div class="sort-header-container__item status">
            <p class="sort-header-container__item__name">Status</p>
        </div>
        <div class="sort-header-container__item amount">
            <p class="sort-header-container__item__name">Amount</p>
        </div>
        <div class="sort-header-container__item download">
            <p class="sort-header-container__item__name">Download</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

@Component
export default class SortingHeader extends Vue {}
</script>

<style scoped lang="scss">
    .sort-header-container {
        display: flex;
        background-color: rgba(245, 246, 250, 0.5);
        width: calc(100% - 60px);
        padding: 0 30px;

        &__item {
            text-align: left;
            min-width: 25%;

            &__name {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 10px;
                color: #afb7c1;
            }
        }
    }

    .description {
        min-width: 31%;
    }

    .status {
        min-width: 12%;
    }

    .amount {
        min-width: 22%;
        margin: 0;
    }

    .download {
        text-align: right;
        margin: 0;
        min-width: 10%;
    }
</style>

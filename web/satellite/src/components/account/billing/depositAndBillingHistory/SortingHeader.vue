// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="sort-header-container">
        <div class="sort-header-container__item date">
            <p class="sort-header-container__item__name">DATE</p>
        </div>
        <div class="sort-header-container__item description">
            <p class="sort-header-container__item__name">DESCRIPTION</p>
        </div>
        <div class="sort-header-container__item status">
            <p class="sort-header-container__item__name">STATUS</p>
        </div>
        <div class="sort-header-container__item amount">
            <p class="sort-header-container__item__name">AMOUNT</p>
        </div>
        <div class="sort-header-container__item download">
            <p class="sort-header-container__item__name">DOWNLOAD</p>
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
        width: 100%;
        padding-bottom: 32px;

        &__item {
            text-align: left;
            min-width: 20%;

            &__name {
                font-family: 'font_medium', sans-serif;
                font-size: 14px;
                line-height: 19px;
                color: #adadad;
                margin: 0;
            }
        }
    }

    .description {
        min-width: 31%;
    }

    .status {
        min-width: 17%;
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

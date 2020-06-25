// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="sort-header-container">
        <div class="sort-header-container__item name">
            <p class="sort-header-container__item__name">BUCKET NAME</p>
        </div>
        <div class="sort-header-container__item">
            <p class="sort-header-container__item__name">STORAGE</p>
        </div>
        <div class="sort-header-container__item">
            <p class="sort-header-container__item__name">BANDWIDTH</p>
        </div>
        <div class="sort-header-container__item">
            <p class="sort-header-container__item__name">OBJECTS</p>
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
        background-color: #fff;
        padding: 35px 40px;
        align-items: center;
        border-bottom: 1px solid #c7cdd2;
        border-radius: 6px 6px 0 0;

        &__item {
            width: 20%;
            margin: 0;
            text-align: left;

            &__name {
                margin: 0;
                font-family: 'font_medium', sans-serif;
                font-size: 14px;
                line-height: 20px;
                color: #acacac;
                letter-spacing: 0.03em;
            }
        }
    }

    .name {
        width: 40%;
    }
</style>

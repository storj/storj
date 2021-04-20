// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="sort-header-container">
        <div class="sort-header-container__item">
            <p class="sort-header-container__item__name">DATE USED</p>
        </div>
        <div class="sort-header-container__item">
            <p class="sort-header-container__item__name">COUPON</p>
        </div>
        <div class="sort-header-container__item">
            <p class="sort-header-container__item__name">EXPIRES</p>
        </div>
        <div class="sort-header-container__item">
            <p class="sort-header-container__item__name">CREDIT AMOUNT</p>
        </div>
        <div class="sort-header-container__item">
            <p class="sort-header-container__item__name">AMOUNT USED</p>
        </div>
        <div class="sort-header-container__item">
            <p class="sort-header-container__item__name">AVAILABLE</p>
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
        margin-bottom: 10px;

        &__item {
            text-align: left;
            min-width: 16.6%;

            &__name {
                font-family: 'font_regular', sans-serif;
                font-weight: 500;
                font-size: 14px;
                line-height: 19px;
                letter-spacing: 0.03em;
                color: #adadad;
                margin: 0;
            }
        }
    }

</style>

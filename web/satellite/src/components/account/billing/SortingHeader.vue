// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="sort-header-container">
        <div class="sort-header-container__item">
            <p class="sort-header-container__item__name">Date</p>
        </div>
        <div class="sort-header-container__item">
            <p class="sort-header-container__item__name">Description</p>
        </div>
        <div class="sort-header-container__item status">
            <p class="sort-header-container__item__name">Status</p>
        </div>
        <div class="sort-header-container__item amount">
            <p class="sort-header-container__item__name">Amount</p>
        </div>
        <div class="sort-header-container__item download"></div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';

    @Component
    export default class SortAccountBillingHeader extends Vue {}
</script>

<style scoped lang="scss">
    .sort-header-container {
        display: flex;
        background-color: rgba(245, 246, 250, 0.5);
        width: calc(100% - 60px);
        padding: 0 30px;
        
        &__item {
            text-align: left;
            width: 35%;
            margin-right: 10px;
            
            &__name {
                font-family: 'font_medium';
                font-size: 16px;
                line-height: 10px;
                color: #AFB7C1;
            }
        }
    }

    .status {
        width: 15%;
    }

    .amount {
        width: 15%;
        margin: 0;
    }

    .download {
        margin: 0;
        min-width: 130px;
    }
</style>

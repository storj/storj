// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="container">
        <p class="container__item">4/20/18 - 5/20/18</p>
        <p class="container__item">STORJ Token Deposit</p>
        <p class="container__item status">Added</p>
        <p class="container__item amount"><b>$25.00</b></p>
        <p class="container__item download">Download <a class="download-link">PDF</a></p>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

@Component
export default class BillingItem extends Vue {}
</script>

<style scoped lang="scss">
    .download-link {
        color: #2683ff;
        font-family: 'font_bold', sans-serif;

        &:hover {
            color: #0059d0;
        }
    }

    .container {
        display: flex;
        padding: 0 30px;
        align-items: center;
        width: calc(100% - 60px);
        border-top: 1px solid rgba(169, 181, 193, 0.3);

        &__item {
            width: 35%;
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            text-align: left;
            margin-right: 10px;
            color: #61666b;
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
        text-align: right;
        min-width: 142px;
    }
</style>

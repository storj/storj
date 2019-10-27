// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="container">
        <p class="container__item">{{billingItem.date()}}</p>
        <p class="container__item">{{billingItem.description}}</p>
        <p class="container__item status">{{billingItem.status}}</p>
        <p class="container__item amount"><b>{{billingItem.amountDollars()}}</b></p>
        <p class="container__item download" v-html="billingItem.downloadLinkHtml()"></p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { BillingHistoryItem } from '@/types/payments';

@Component
export default class BillingItem extends Vue {
    @Prop({default: new BillingHistoryItem()})
    private readonly billingItem: BillingHistoryItem;
}
</script>

<style scoped lang="scss">
    .download-link {
        color: #2683FF;
        font-family: 'font_bold';

        &:hover {
            color: #0059D0;
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
            font-family: 'font_medium';
            font-size: 16px;
            text-align: left;
            margin-right: 10px;
            color: #61666B;
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

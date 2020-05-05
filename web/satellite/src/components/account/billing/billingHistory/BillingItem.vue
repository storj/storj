// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="container">
        <BillingHistoryItemDate
            class="container__item"
            :start="billingItem.start"
            :expiration="billingItem.end"
            :type="billingItem.type"
        />
        <p class="container__item description">{{ billingItem.description }}</p>
        <p class="container__item status">{{ billingItem.formattedStatus }}</p>
        <p class="container__item amount">
            <b>
                {{ billingItem.quantity.currency }}
                <span v-if="billingItem.type === 1">
                    {{ billingItem.quantity.received.toFixed(2) }}
                </span>
                <span v-else>
                    {{ billingItem.quantity.total.toFixed(2) }}
                </span>
            </b>
            <span v-if="billingItem.type === 1">
                of <b>{{ billingItem.quantity.total.toFixed(2) }}</b>
            </span>
        </p>
        <p class="container__item download" v-html="billingItem.downloadLinkHtml()"></p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import BillingHistoryItemDate from '@/components/account/billing/billingHistory/BillingHistoryItemDate.vue';

import { BillingHistoryItem } from '@/types/payments';

@Component({
    components: {
        BillingHistoryItemDate,
    },
})
export default class BillingItem extends Vue {
    @Prop({default: () => new BillingHistoryItem()})
    private readonly billingItem: BillingHistoryItem;
}
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
            min-width: 25%;
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            text-align: left;
            color: #61666b;
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
        margin: 0;
        text-align: right;
        width: 10%;
        min-width: 10%;
    }

    .row {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: flex-start;
        width: 175px;
    }
</style>

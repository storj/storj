// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="container">
        <Countdown
            class="container__item"
            :start-date="billingItem.start"
            :expiration-date="billingItem.end"
            :type="billingItem.type"
        ></Countdown>
        <p class="container__item description">{{billingItem.description}}</p>
        <p class="container__item status">{{billingItem.status}}</p>
        <p class="container__item amount">
            <b>
                {{billingItem.quantity.currency}}
                <span v-if="billingItem.quantity.received">
                    {{billingItem.quantity.received}}
                </span>
                <span v-else>
                    {{billingItem.quantity.total}}
                </span>
            </b>
            <span v-if="billingItem.quantity.received">
                 of {{billingItem.quantity.total}}
            </span>
        </p>
        <p class="container__item download" v-html="billingItem.downloadLinkHtml()"></p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import Countdown from '@/components/account/billing/depositAndBilling/Countdown.vue';

import { BillingHistoryItem } from '@/types/payments';

@Component({
    components: {
        Countdown,
    },
})
export default class BillingItem extends Vue {
    @Prop({default: new BillingHistoryItem()})
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
            width: 20%;
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            text-align: left;
            color: #61666b;
        }
    }

    .description {
        width: 31%;
    }

    .status {
        width: 12%;
    }

    .amount {
        width: 27%;
        margin: 0;
    }

    .download {
        margin: 0;
        text-align: right;
        min-width: 142px;
        width: 10%;
    }

    .row {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: flex-start;
        width: 175px;
    }
</style>

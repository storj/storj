// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="container">
        <PaymentsHistoryItemDate
            class="container__item date"
            :start="billingItem.start"
            :expiration="billingItem.end"
            :type="billingItem.type"
            :status="billingItem.status"
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
        <p class="container__item download">
            <a v-if="billingItem.link" class="download-link" target="_blank" :href="billingItem.link">{{ billingItem.label }}</a>
        </p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { PaymentsHistoryItem } from '@/types/payments';

import PaymentsHistoryItemDate from '@/components/account/billing/depositAndBillingHistory/PaymentsHistoryItemDate.vue';

// @vue/component
@Component({
    components: {
        PaymentsHistoryItemDate,
    },
})
export default class PaymentsItem extends Vue {
    @Prop({ default: () => new PaymentsHistoryItem() })
    private readonly billingItem: PaymentsHistoryItem;
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
        align-items: center;
        width: 100%;
        border-top: 1px solid #c7cdd2;

        &__item {
            min-width: 20%;
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            text-align: left;
            color: #768394;
            margin: 30px 0;
        }
    }

    .date {
        font-family: 'font_bold', sans-serif;
        margin: 0;
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

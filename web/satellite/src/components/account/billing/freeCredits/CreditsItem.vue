// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="container">
        <p class="container__item">{{ creditType }}</p>
        <p class="container__item">{{ expiration }}</p>
        <p class="container__item">{{ memoryAmount }} GB ({{ creditsItem.amount | centsToDollars }})</p>
        <p class="container__item available">{{ creditsItem.remaining | centsToDollars }}</p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import PaymentsHistoryItemDate from '@/components/account/billing/depositAndBillingHistory/PaymentsHistoryItemDate.vue';

import { PaymentsHistoryItem, PaymentsHistoryItemType } from '@/types/payments';
import { MONTHS_NAMES } from '@/utils/constants/date';

@Component({
    components: {
        PaymentsHistoryItemDate,
    },
})
export default class CreditsItem extends Vue {
    @Prop({default: () => new PaymentsHistoryItem()})
    private readonly creditsItem: PaymentsHistoryItem;

    /**
     * Return credit type string depending on item type.
     */
    public get creditType(): string {
        const trial = 'Trial Credit';
        const referral = 'Referral Credit';

        if (this.creditsItem.type === PaymentsHistoryItemType.Coupon) {
            return trial;
        }

        return referral;
    }

    /**
     * Returns memory amount depending on item's money amount.
     */
    public get memoryAmount(): number {
        const gbPrice: number = 5.5; // in cents.

        return Math.floor(this.creditsItem.amount / gbPrice);
    }

    /**
     * Returns formatted string of expiration date.
     */
    public get expiration(): string {
        const monthNumber = this.creditsItem.end.getUTCMonth();
        const year = this.creditsItem.end.getUTCFullYear();

        return `${MONTHS_NAMES[monthNumber]} ${year}`;
    }
}
</script>

<style scoped lang="scss">
    .container {
        display: flex;
        align-items: center;
        width: 100%;

        &__item {
            min-width: 28%;
            font-family: 'font_regular', sans-serif;
            text-align: left;
            margin: 10px 0;
            font-size: 16px;
            line-height: 19px;
            color: #354049;
        }
    }

    .available {
        min-width: 16%;
        text-align: right;
    }
</style>

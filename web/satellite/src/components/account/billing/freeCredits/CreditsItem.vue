// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="container">
        <p class="container__item">{{ startDate }}</p>
        <p class="container__item coupon">{{ creditType }}</p>
        <p :class="{'expired' : expirationCheck}" class="container__item">{{ expiration }}</p>
        <p class="container__item">{{  creditsItem.amount | centsToDollars }}</p>
        <p class="container__item">{{ amountUsed | centsToDollars }}</p>
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
        const trial = 'Free Tier Credit';

        if (this.creditsItem.type === PaymentsHistoryItemType.Coupon) {
            return trial;
        }

        return '';
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
        if (!this.creditsItem.hasExpiration) {
            return 'Never expires';
        }

        const monthNumber = this.creditsItem.end.getUTCMonth();
        const year = this.creditsItem.end.getUTCFullYear();

        return `${MONTHS_NAMES[monthNumber]} ${year}`;
    }

    /**
     * Returns formatted string of start date.
     */
    public get startDate(): string {
        const monthNumber = this.creditsItem.start.getUTCMonth();
        const year = this.creditsItem.start.getUTCFullYear();

        return `${MONTHS_NAMES[monthNumber]} ${year}`;
    }

    /**
     * Returns remaining amount
     */
    public get amountUsed(): number {
        const amount = this.creditsItem.amount;
        const remaining = this.creditsItem.remaining;

        return amount - remaining;
    }

    /**
     * Checks for coupon expiration.
     */
    public get expirationCheck(): boolean {
        return this.creditsItem.hasExpiration && this.creditsItem.end.getTime() < new Date().getTime();
    }
}
</script>

<style scoped lang="scss">
    .container {
        display: flex;
        width: 100%;

        &__item {
            min-width: 16.6%;
            font-family: 'font_regular', sans-serif;
            text-align: left;
            font-size: 16px;
            line-height: 19px;
            color: #354049;
        }

        &__item.coupon {
            font-family: 'font_bold', sans-serif;
        }

        &__item.expired {
            color: #ce3030;
        }
    }

</style>

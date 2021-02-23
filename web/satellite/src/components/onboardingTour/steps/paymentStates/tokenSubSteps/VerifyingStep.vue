// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./verifyingStep.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BackImage from '@/../static/images/account/billing/back.svg';
import VerifyingImage from '@/../static/images/onboardingTour/verifying.svg';

import { PaymentsHistoryItem, PaymentsHistoryItemType } from '@/types/payments';

@Component({
    components: {
        VerifyingImage,
        BackImage,
    },
})

export default class VerifyingStep extends Vue {
    /**
     * Returns last coin payment transaction's link;
     */
    public get lastTransactionLink(): string {
        const transactions: PaymentsHistoryItem[] = this.$store.state.paymentsModule.paymentsHistory.filter((item: PaymentsHistoryItem) => {
            return item.type === PaymentsHistoryItemType.Transaction;
        });

        return transactions[0].link;
    }

    /**
     * Holds logic for button click.
     * Sets area to default state.
     */
    public onBackClick(): void {
        this.$emit('setDefaultState');
    }
}
</script>

<style scoped lang="scss" src="./verifyingStep.scss"></style>

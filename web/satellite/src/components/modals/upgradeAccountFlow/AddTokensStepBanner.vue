// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="banner" :class="{success: isSuccess}">
        <template v-if="isDefault">
            <h2 class="banner__title">Send only STORJ Tokens to this deposit address.</h2>
            <p class="banner__message">
                Sending anything else may result in the loss of your deposit.
            </p>
        </template>
        <template v-if="isPending">
            <div class="banner__row">
                <PendingIcon />
                <p class="banner__message">
                    <b>{{ stillPendingTransactions.length }} transaction{{ stillPendingTransactions.length > 1 ? 's' : '' }} pending...</b>
                    {{ txWithLeastConfirmations.confirmations }} of {{ neededConfirmations }} confirmations
                </p>
            </div>
        </template>
        <template v-if="isSuccess">
            <div class="banner__row">
                <CheckIcon />
                <p class="banner__message">
                    Successful deposit of {{ totalValueCounter('tokenValue') }} STORJ tokens.
                    You received an additional bonus of {{ totalValueCounter('bonusTokens') }} STORJ tokens.
                </p>
            </div>
        </template>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { PaymentStatus, PaymentWithConfirmations } from '@/types/payments';
import { useConfigStore } from '@/store/modules/configStore';

import PendingIcon from '@/../static/images/modals/upgradeFlow/pending.svg';
import CheckIcon from '@/../static/images/modals/upgradeFlow/check.svg';

const configStore = useConfigStore();

const props = defineProps<{
    isDefault: boolean
    isPending: boolean
    isSuccess: boolean
    pendingPayments: PaymentWithConfirmations[]
}>();

/**
 * Returns an array of still pending transactions to correctly display confirmations count.
 */
const stillPendingTransactions = computed((): PaymentWithConfirmations[] => {
    return props.pendingPayments.filter(p => p.status === PaymentStatus.Pending);
});

/**
 * Returns transaction with the least confirmations count.
 */
const txWithLeastConfirmations = computed((): PaymentWithConfirmations => {
    return stillPendingTransactions.value.reduce((minTx: PaymentWithConfirmations, currentTx: PaymentWithConfirmations) => {
        if (currentTx.confirmations < minTx.confirmations) {
            return currentTx;
        }
        return minTx;
    }, props.pendingPayments[0]);
});

/**
 * Returns needed confirmations count for each transaction from config store.
 */
const neededConfirmations = computed((): number => {
    return configStore.state.config.neededTransactionConfirmations;
});

/**
 * Calculates total count of provided payment field from the list (i.e. tokenValue or bonusTokens).
 */
function totalValueCounter(field: keyof PaymentWithConfirmations): string {
    return props.pendingPayments.reduce((acc: number, curr: PaymentWithConfirmations) => {
        return acc + (curr[field] as number);
    }, 0).toLocaleString(undefined, { maximumFractionDigits: 2 });
}
</script>

<style scoped lang="scss">
.banner {
    margin-top: 16px;
    padding: 16px;
    background: var(--c-yellow-1);
    border: 1px solid var(--c-yellow-2);
    box-shadow: 0 7px 20px rgb(0 0 0 / 15%);
    border-radius: 10px;

    &__title,
    &__message {
        font-size: 14px;
        line-height: 20px;
        color: var(--c-black);
        text-align: left;
    }

    &__title {
        font-family: 'font_bold', sans-serif;
    }

    &__row {
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 16px;

        svg {
            min-width: 32px;
        }
    }
}

.success {
    background: var(--c-green-4);
    border-color: var(--c-green-5);
}
</style>

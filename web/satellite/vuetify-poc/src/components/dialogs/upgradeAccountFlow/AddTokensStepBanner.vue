// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        class="mt-3"
        density="compact"
        variant="tonal"
        :type="isSuccess ? 'success' : 'warning'"
    >
        <template #prepend>
            <v-icon v-if="isDefault" icon="mdi-information-outline" />
            <v-icon v-if="isPending" icon="mdi-clock-outline" />
            <v-icon v-if="isSuccess" icon="mdi-check-circle-outline" />
        </template>

        <template #text>
            <p v-if="isDefault">
                <span class="font-weight-bold d-block">Send only STORJ Tokens to this deposit address.</span>
                <span>Sending anything else may result in the loss of your deposit.</span>
            </p>

            <div v-if="isPending">
                <p class="banner__message">
                    <b>{{ stillPendingTransactions.length }} transaction{{ stillPendingTransactions.length > 1 ? 's' : '' }} pending...</b>
                    {{ txWithLeastConfirmations.confirmations }} of {{ neededConfirmations }} confirmations
                </p>
            </div>

            <div v-if="isSuccess" class="banner__row">
                <p class="banner__message">
                    Successful deposit of {{ totalValueCounter('tokenValue') }} STORJ tokens.
                    You received an additional bonus of {{ totalValueCounter('bonusTokens') }} STORJ tokens.
                </p>
            </div>
        </template>
    </v-alert>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VAlert, VIcon } from 'vuetify/components';

import { PaymentStatus, PaymentWithConfirmations } from '@/types/payments';
import { useConfigStore } from '@/store/modules/configStore';

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
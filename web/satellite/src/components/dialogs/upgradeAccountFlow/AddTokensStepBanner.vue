// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        class="mt-3 mb-2"
        density="compact"
        variant="tonal"
        :type="isSuccess ? 'success' : 'warning'"
    >
        <template #prepend>
            <v-icon v-if="isDefault" :icon="mdiInformationOutline" />
            <v-icon v-if="isPending" :icon="mdiClockOutline" />
            <v-icon v-if="isSuccess" :icon="mdiCheckCircleOutline" />
        </template>

        <template #text>
            <p v-if="isDefault">
                <span class="font-weight-bold d-block">Send STORJ tokens via the Ethereum network or zkSync Era.</span>
                <span>
                    If you send any other kind of token, or use any other network, you may lose your deposit.
                </span>
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
import { mdiCheckCircleOutline, mdiClockOutline, mdiInformationOutline } from '@mdi/js';

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

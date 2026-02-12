// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        v-if="alertVisible"
        :closable="!isLoading"
        variant="outlined"
        type="warning"
        title="Payment could not be processed"
        class="my-4 pb-4"
        border
    >
        <p class="mt-1">{{ message }} <a :href="configStore.supportUrl" target="_blank" rel="noopener">Need help?</a></p>
        <v-btn
            class="d-block mt-2"
            density="comfortable"
            :loading="isLoading"
            @click="retryPayment"
        >
            Pay Invoice
        </v-btn>
    </v-alert>
</template>

<script setup lang="ts">
import { VAlert, VBtn } from 'vuetify/components';
import { computed } from 'vue';

import { useBillingStore } from '@/store/modules/billingStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useNotify } from '@/composables/useNotify';
import { centsToDollars } from '@/utils/strings';
import { useLoading } from '@/composables/useLoading';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { PaymentsHistoryItem } from '@/types/payments';

const billingStore = useBillingStore();
const configStore = useConfigStore();
const usersStore = useUsersStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const billingEnabled = computed<boolean>(() => configStore.getBillingEnabled(usersStore.state.user));
const failedInvoice = computed<PaymentsHistoryItem | null>(() => billingStore.state.failedInvoice);

const alertVisible = computed<boolean>(() => billingEnabled.value && failedInvoice.value !== null);

const message = computed<string>(() => {
    if (!failedInvoice.value) return '';

    const amount = centsToDollars(failedInvoice.value.amount);
    const period = new Intl.DateTimeFormat('en', { month: 'long', year: 'numeric' }).format(failedInvoice.value.start);

    return `Please retry your payment (${amount} for ${period}) to avoid service interruption.`;
});

function retryPayment(): void {
    withLoading(async () => {
        try {
            await billingStore.attemptPayments();
            notify.success('Payment successful');
            await billingStore.getFailedInvoice();
        } catch (error) {
            // API payment failed, open external Stripe payment page
            if (failedInvoice.value?.payLink) {
                window.open(failedInvoice.value.payLink, '_blank', 'noopener,noreferrer');
                return;
            }

            notify.notifyError(error, AnalyticsErrorEventSource.RETRY_PAYMENT_BANNER);
        }
    });
}
</script>

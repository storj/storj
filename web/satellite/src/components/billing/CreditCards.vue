// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-col v-for="card in creditCards" :key="card.id" cols="12" sm="12" md="6" lg="6" xl="4">
        <CreditCardComponent :card="card" />
    </v-col>

    <v-col cols="12" sm="12" md="6" lg="6" xl="4">
        <AddCreditCardComponent />
    </v-col>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';
import { VCol } from 'vuetify/components';

import { useBillingStore } from '@/store/modules/billingStore';
import { CreditCard } from '@/types/payments';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';

import CreditCardComponent from '@/components/billing/CreditCardComponent.vue';
import AddCreditCardComponent from '@/components/billing/AddCreditCardComponent.vue';

const notify = useNotify();

const billingStore = useBillingStore();

const creditCards = computed((): CreditCard[] =>
    billingStore.state.creditCards
        .slice()
        .sort((a, b) => Number(b.isDefault) - Number(a.isDefault)),
);

onMounted(() => {
    billingStore.getCreditCards().catch(error => notify.notifyError(error, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB));
});
</script>

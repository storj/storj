// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        v-if="alertVisible"
        closable
        variant="outlined"
        :type="cardExpired ? 'error' : 'info'"
        :title="`Credit Card ${cardExpired ? 'Expired' : 'Expiring Soon'}`"
        class="my-4 pb-4"
        border
    >
        <p class="mt-1">{{ message }}</p>
        <v-btn
            class="d-block mt-2"
            :color="cardExpired ? 'error' : 'primary'"
            density="comfortable"
            @click="goToBilling"
        >
            Update Payment Method
        </v-btn>
    </v-alert>
</template>

<script setup lang="ts">
import { VAlert, VBtn } from 'vuetify/components';
import { computed } from 'vue';
import { useRouter } from 'vue-router';

import { useAppStore } from '@/store/modules/appStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { ROUTES } from '@/router';
import { useConfigStore } from '@/store/modules/configStore';
import { useUsersStore } from '@/store/modules/usersStore';

const router = useRouter();

const appStore = useAppStore();
const billingStore = useBillingStore();
const usersStore = useUsersStore();
const configStore = useConfigStore();

const billingEnabled = computed<boolean>(() => configStore.getBillingEnabled(usersStore.state.user));
const cardExpiring = computed<boolean>(() => billingStore.defaultCard.isExpiring);
const cardExpired = computed<boolean>(() => billingStore.defaultCard.isExpired);

const alertVisible = computed<boolean>(() =>
    billingEnabled.value &&
    appStore.state.hasJustLoggedIn &&
    billingStore.defaultCard.id !== '' &&
    (cardExpired.value || cardExpiring.value));

const message = computed<string>(() => {
    if (cardExpired.value) {
        return 'Your default credit card has expired. To avoid any interruption in service, please update your payment information as soon as possible.';
    }
    return 'Your default credit card will expire soon. To avoid any interruption in service, please update your payment information when possible.';
});

function goToBilling(): void {
    router.push({
        path: ROUTES.Account.with(ROUTES.Billing).path,
        query: {
            tab: 'payment-methods',
        },
    });
}
</script>

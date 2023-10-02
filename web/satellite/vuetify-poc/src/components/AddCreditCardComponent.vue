// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card title="Add Card" variant="flat" :border="true" rounded="xlg">
        <v-card-text>
            <v-btn v-if="!isCardInputShown" variant="outlined" color="default" size="small" class="mr-2" @click="isCardInputShown = true">+ Add New Card</v-btn>
            
            <template v-else>
                <StripeCardInput
                    ref="stripeCardInput"
                    :on-stripe-response-callback="addCardToDB"
                />
            </template>

            <template v-if="isCardInputShown">
                <v-btn
                    color="primary" size="small" class="mr-2"
                    :disabled="isLoading"
                    :loading="isLoading"
                    @click="onSaveCardClick"
                >
                    Add Card
                </v-btn>
                <v-btn
                    variant="outlined" color="default" size="small" class="mr-2"
                    :disabled="isLoading"
                    :loading="isLoading"
                    @click="isCardInputShown = false"
                >
                    Cancel
                </v-btn>
            </template>

        </v-card-text>
    </v-card>
</template>

<script setup lang="ts">
import { VBtn, VCard, VCardText, VCardActions } from 'vuetify/components';
import { ref } from 'vue';

import { useUsersStore } from '@/store/modules/usersStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { useBillingStore } from '@/store/modules/billingStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import StripeCardInput from '@/components/account/billing/paymentMethods/StripeCardInput.vue';

interface StripeForm {
  onSubmit(): Promise<void>;
}

const usersStore = useUsersStore();
const notify = useNotify();
const billingStore = useBillingStore();
const { isLoading } = useLoading();

const stripeCardInput = ref<typeof StripeCardInput & StripeForm | null>(null);

const isCardInputShown = ref(false);

/**
 * Provides card information to Stripe.
 */
async function onSaveCardClick(): Promise<void> {
    if (isLoading.value || !stripeCardInput.value) return;

    try {
        await stripeCardInput.value.onSubmit();
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
    } finally {
        isLoading.value = false;
    }
}

/**
 * Adds card after Stripe confirmation.
 *
 * @param token from Stripe
 */
async function addCardToDB(token: string): Promise<void> {
    isLoading.value = true;
    try {
        await billingStore.addCreditCard(token);
        notify.success('Card successfully added');
        isCardInputShown.value = false;
        isLoading.value = false;

        // We fetch User one more time to update their Paid Tier status.
        usersStore.getUser().catch();

        billingStore.getCreditCards().catch();
    } catch (error) {
        isLoading.value = false;
        notify.notifyError(error, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
    }
}
</script>
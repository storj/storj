// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card title="Add Card" variant="flat" :border="true" rounded="xlg">
        <v-card-text>
            <v-btn v-if="!isCardInputShown" variant="outlined" color="default" size="small" class="mr-2" @click="isCardInputShown = true">+ Add New Card</v-btn>

            <template v-else>
                <StripeCardElement
                    v-if="paymentElementEnabled"
                    ref="stripeCardInput"
                />
                <StripeCardInput
                    v-else
                    ref="stripeCardInput"
                    :on-stripe-response-callback="addCardToDB"
                />
            </template>

            <div v-if="isCardInputShown" class="mt-4">
                <v-btn
                    color="primary" size="small" class="mr-2"
                    :loading="isLoading"
                    @click="onSaveCardClick"
                >
                    Add Card
                </v-btn>
                <v-btn
                    variant="outlined" color="default" size="small" class="mr-2"
                    :disabled="isLoading"
                    @click="isCardInputShown = false"
                >
                    Cancel
                </v-btn>
            </div>
        </v-card-text>
    </v-card>
</template>

<script setup lang="ts">
import { VBtn, VCard, VCardText } from 'vuetify/components';
import { computed, ref } from 'vue';

import { useUsersStore } from '@/store/modules/usersStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { useBillingStore } from '@/store/modules/billingStore';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import StripeCardElement from '@/components/StripeCardElement.vue';
import StripeCardInput from '@/components/StripeCardInput.vue';

interface StripeForm {
    onSubmit(): Promise<string>;
}

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const usersStore = useUsersStore();
const notify = useNotify();
const billingStore = useBillingStore();
const { withLoading, isLoading } = useLoading();

const stripeCardInput = ref<StripeForm | null>(null);

const isCardInputShown = ref(false);

/**
 * Indicates whether stripe payment element is enabled.
 */
const paymentElementEnabled = computed(() => {
    return configStore.state.config.stripePaymentElementEnabled;
});
/**
 * Provides card information to Stripe.
 */
function onSaveCardClick(): void {
    withLoading(async () => {
        if (!stripeCardInput.value) return;

        try {
            const response = await stripeCardInput.value.onSubmit();
            await addCardToDB(response);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
        }
    });
}

/**
 * Adds card after Stripe confirmation.
 *
 * @param res - the response from stripe. Could be a token or a payment method id.
 * depending on the paymentElementEnabled flag.
 */
async function addCardToDB(res: string) {
    try {
        const action = paymentElementEnabled.value ? billingStore.addCardByPaymentMethodID : billingStore.addCreditCard;
        await action(res);
        notify.success('Card successfully added');
        isCardInputShown.value = false;

        analyticsStore.eventTriggered(AnalyticsEvent.CREDIT_CARD_ADDED_FROM_BILLING);

        // We fetch User one more time to update their Paid Tier status.
        usersStore.getUser().catch((_) => {});

        billingStore.getCreditCards().catch((_) => {});
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
    }
}
</script>

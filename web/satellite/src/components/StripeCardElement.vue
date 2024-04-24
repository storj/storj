// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div id="payment-element">
        <!-- A Stripe Element will be inserted here. -->
    </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue';
import { loadStripe } from '@stripe/stripe-js/pure';
import { Stripe, StripeElements, StripePaymentElement, StripeElementsOptionsMode } from '@stripe/stripe-js';
import { useTheme } from 'vuetify';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useConfigStore } from '@/store/modules/configStore';

const configStore = useConfigStore();

const notify = useNotify();
const theme = useTheme();

/**
 * Stripe elements is used to create 'Add Card' form.
 */
const paymentElement = ref<StripePaymentElement>();
/**
 * Stripe library.
 */
const stripe = ref<Stripe | null>(null);
const elements = ref<StripeElements | null>(null);

/**
 * Stripe initialization.
 */
async function initStripe(): Promise<void> {
    const stripePublicKey = configStore.state.config.stripePublicKey;

    try {
        stripe.value = await loadStripe(stripePublicKey);
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.BILLING_STRIPE_CARD_INPUT);
        return;
    }

    if (!stripe.value) {
        notify.error('Unable to initialize stripe', AnalyticsErrorEventSource.BILLING_STRIPE_CARD_INPUT);
        return;
    }

    const options: StripeElementsOptionsMode = {
        mode: 'setup',
        currency: 'usd',
        paymentMethodCreation: 'manual',
        paymentMethodTypes: ['card'],
        appearance: {
            theme: theme.global.current.value.dark ? 'night' : 'stripe',
            labels: 'floating',
        },
    };
    elements.value = stripe.value?.elements(options);

    if (!elements.value) {
        notify.error('Unable to instantiate elements', AnalyticsErrorEventSource.BILLING_STRIPE_CARD_INPUT);
        return;
    }

    paymentElement.value = elements.value.create('payment');
    if (!paymentElement.value) {
        notify.error('Unable to create card element', AnalyticsErrorEventSource.BILLING_STRIPE_CARD_INPUT);
        return;
    }

    paymentElement.value?.mount('#payment-element');
}

/**
 * Fires stripe event after all inputs are filled.
 * This method is called from the parent component.
 *
 * @returns {string} Payment method id.
 * @throws {Error}
 */
async function onSubmit(): Promise<string> {

    if (!(stripe.value && elements.value && paymentElement.value)) {
        throw new Error('Stripe is not initialized');
    }

    // Trigger form validation
    const res = await elements.value.submit();
    if (res.error) {
        throw new Error(res.error.message ?? 'There is an issue with the card');
    }

    // Create the PaymentMethod using the details collected by the Payment Element
    const { error, paymentMethod } = await stripe.value.createPaymentMethod({
        elements: elements.value,
    });
    if (error) {
        throw new Error(error.message ?? 'There is an issue with the card');
    }

    if (paymentMethod.card?.funding === 'prepaid') {
        throw new Error('Prepaid cards are not supported');
    }
    return paymentMethod.id;
}

watch(() => theme.global.current.value.dark, isDarkTheme => {
    elements.value?.update({
        appearance: {
            theme: isDarkTheme ? 'night' : 'stripe',
            labels: 'floating',
        },
    });
});

/**
 * Stripe library loading and initialization.
 */
onMounted(() => {
    initStripe();
});

defineExpose({
    onSubmit,
});
</script>
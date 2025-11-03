// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-progress-linear rounded indeterminate :active="isLoading" />
    <div id="payment-element">
        <!-- A Stripe Element will be inserted here. -->
    </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { loadStripe } from '@stripe/stripe-js/pure';
import {
    Stripe,
    StripeElements,
    StripeElementsOptionsMode,
    StripePaymentElement,
} from '@stripe/stripe-js';
import { useTheme } from 'vuetify';
import { VProgressLinear } from 'vuetify/components';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { useConfigStore } from '@/store/modules/configStore';
import { useBillingStore } from '@/store/modules/billingStore';

const configStore = useConfigStore();
const billingStore = useBillingStore();

const notify = useNotify();
const theme = useTheme();

const isLoading = ref(true);

const emit = defineEmits<{
    ready: [];
}>();

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
        const options: StripeElementsOptionsMode = {
            currency:'usd',
            appearance: {
                theme: theme.global.current.value.dark ? 'night' : 'stripe',
                labels: 'floating',
            },
            // @ts-expect-error clientSecret can be assigned a string but is defined as undefined in the type.
            clientSecret: await billingStore.getCardSetupSecret(),
        };

        if (!stripe.value) {
            // load stripe library
            stripe.value = await loadStripe(stripePublicKey);
            if (!stripe.value) throw new Error('Unable to initialize stripe');
        }

        // initialize stripe elements
        elements.value = stripe.value?.elements(options);
        if (!elements.value) throw new Error('Unable to instantiate elements');

        // create payment element
        paymentElement.value?.off('ready');
        paymentElement.value = elements.value.create('payment');
        if (!paymentElement.value) throw new Error('Unable to create card element');

        paymentElement.value?.mount('#payment-element');
        paymentElement.value?.on('ready', () => {
            isLoading.value = false;
            emit('ready');
        });
    } catch (error) {
        isLoading.value = false;
        notify.error(error.message, AnalyticsErrorEventSource.BILLING_STRIPE_CARD_INPUT);
        return;
    }
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
    const stripeValue = stripe.value;
    const elementsValue = elements.value;

    // Trigger form validation.
    const res = await elements.value.submit();
    if (res.error) {
        throw new Error(res.error.message ?? 'There is an issue with the card');
    }

    const { error, setupIntent } = await stripeValue.confirmSetup({
        elements: elementsValue,
        redirect: 'if_required',
        confirmParams: {
            expand: ['payment_method'],
        },
    });

    if (error || !setupIntent.payment_method || setupIntent.status !== 'succeeded') {
        throw new Error(error?.message ?? 'There is an issue with the card');
    }

    const paymentMethod = setupIntent.payment_method;

    if (typeof paymentMethod === 'string') {
        return paymentMethod;
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

onBeforeUnmount(() => {
    paymentElement.value?.off('ready');
});

defineExpose({
    onSubmit,
    initStripe,
});
</script>
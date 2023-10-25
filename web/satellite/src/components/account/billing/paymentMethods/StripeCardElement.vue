// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <form id="payment-form">
        <div class="form-row">
            <div id="payment-element">
                <!-- A Stripe Element will be inserted here. -->
            </div>
            <div id="card-errors" role="alert" />
        </div>
    </form>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue';
import { loadStripe } from '@stripe/stripe-js/pure';
import { Stripe, StripeElements, StripePaymentElement } from '@stripe/stripe-js';
import { StripeElementsOptionsMode } from '@stripe/stripe-js/types/stripe-js/elements-group';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useConfigStore } from '@/store/modules/configStore';
import { useLoading } from '@/composables/useLoading';
import { useBillingStore } from '@/store/modules/billingStore';

const configStore = useConfigStore();
const billingStore = useBillingStore();
const notify = useNotify();
const { withLoading, isLoading } = useLoading();

const props = withDefaults(defineProps<{
    isDarkTheme?: boolean
}>(), {
    isDarkTheme: false,
});

const emit = defineEmits<{
  (e: 'pmCreated', pmID: string): void
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
            theme: props.isDarkTheme ? 'night' : 'stripe',
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
 */
function onSubmit(): void {

    const displayError: HTMLElement = document.getElementById('card-errors') as HTMLElement;

    withLoading(async () => {
        if (!(stripe.value && elements.value && paymentElement.value)) {
            notify.error('Stripe is not initialized', AnalyticsErrorEventSource.BILLING_STRIPE_CARD_INPUT);
            return;
        }

        // Trigger form validation
        const res = await elements.value.submit();
        if (res.error) {
            displayError.textContent = res.error.message ?? '';
            return;
        }

        // Create the PaymentMethod using the details collected by the Payment Element
        const { error, paymentMethod } = await stripe.value.createPaymentMethod({
            elements: elements.value,
        });
        if (error) {
            displayError.textContent = error.message ?? '';
            return;
        }

        if (paymentMethod.card?.funding === 'prepaid') {
            displayError.textContent = 'Prepaid cards are not supported';
            return;
        }

        emit('pmCreated', paymentMethod.id);
    });
}

watch(() => props.isDarkTheme, isDarkTheme => {
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

<style scoped lang="scss">
    .StripeElement {
        box-sizing: border-box;
        width: 100%;
        padding-bottom: 14px;
        border-radius: 4px;
        transition: box-shadow 150ms ease;
    }

    .StripeElement--focus {
        box-shadow: 0 1px 3px 0 #cfd7df;
    }

    .StripeElement--invalid {
        border-color: #fa755a;
    }

    .StripeElement--webkit-autofill {
        background-color: #fefde5 !important;
    }

    .form-row {
        width: 100%;
    }

    #card-errors {
        text-align: left;
        font-family: 'font-medium', sans-serif;
        color: var(--c-red-2);
    }
</style>

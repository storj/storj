// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <form id="payment-form">
        <div class="form-row">
            <div id="card-element">
                <!-- A Stripe Element will be inserted here. -->
            </div>
            <div id="card-errors" role="alert" />
        </div>
    </form>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { loadStripe } from '@stripe/stripe-js/pure';
import {
    Stripe,
    StripeCardElement,
    StripeCardElementChangeEvent,
    TokenResult,
} from '@stripe/stripe-js';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useConfigStore } from '@/store/modules/configStore';

const configStore = useConfigStore();
const notify = useNotify();

const props = withDefaults(defineProps<{
    onStripeResponseCallback: (tokenId: unknown) => Promise<void>,
}>(), {
    onStripeResponseCallback: () => Promise.reject('onStripeResponse is not reinitialized'),
});

const isLoading = ref<boolean>(false);
/**
 * Stripe elements is used to create 'Add Card' form.
 */
const cardElement = ref<StripeCardElement>();
/**
 * Stripe library.
 */
const stripe = ref<Stripe | null>(null);

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

    const elements = stripe.value?.elements();
    if (!elements) {
        notify.error('Unable to instantiate elements', AnalyticsErrorEventSource.BILLING_STRIPE_CARD_INPUT);
        return;
    }

    cardElement.value = elements.create('card');
    if (!cardElement.value) {
        notify.error('Unable to create card element', AnalyticsErrorEventSource.BILLING_STRIPE_CARD_INPUT);
        return;
    }

    cardElement.value?.mount('#card-element');
    cardElement.value?.on('change', (event: StripeCardElementChangeEvent) => {
        const displayError: HTMLElement = document.getElementById('card-errors') as HTMLElement;
        if (event.error) {
            displayError.textContent = event.error.message;
        } else {
            displayError.textContent = '';
        }
    });
}

/**
 * Event after card adding.
 * Returns token to callback and clears card input
 *
 * @param result stripe response
 */
async function onStripeResponse(result: TokenResult): Promise<void> {
    if (result.error) {
        throw result.error;
    }

    if (result.token.card?.funding === 'prepaid') {
        notify.error('Prepaid cards are not supported', AnalyticsErrorEventSource.BILLING_STRIPE_CARD_INPUT);
        return;
    }

    await props.onStripeResponseCallback(result.token.id);
    cardElement.value?.clear();
}

/**
 * Fires stripe event after all inputs are filled.
 */
async function onSubmit(): Promise<void> {
    if (!(stripe.value && cardElement.value)) {
        notify.error('Stripe is not initialized', AnalyticsErrorEventSource.BILLING_STRIPE_CARD_INPUT);
        return;
    }

    if (isLoading.value) return;

    isLoading.value = true;

    try {
        await stripe.value.createToken(cardElement.value).then(onStripeResponse);
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.BILLING_STRIPE_CARD_INPUT);
    }

    isLoading.value = false;
}

/**
 * Stripe library loading and initialization.
 */
onMounted(() => {
    initStripe();
});

/**
 * Clears listeners.
 */
onBeforeUnmount(() => {
    cardElement.value?.off('change');
});

defineExpose({
    onSubmit,
});
</script>

<style scoped lang="scss">
    .StripeElement {
        box-sizing: border-box;
        width: 100%;
        padding: 13px 12px;
        border: 1px solid var(--c-grey-2);
        border-radius: 4px;
        background-color: white;
        box-shadow: 0 2px 5px 0 rgb(50 50 93 / 7%);
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

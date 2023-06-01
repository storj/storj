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

import { LoadScript } from '@/utils/loadScript';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useConfigStore } from '@/store/modules/configStore';

interface StripeResponse {
    error: string
    token: {
        id: unknown
        card: {
            funding : string
        }
    }
}

const configStore = useConfigStore();
const notify = useNotify();

const props = withDefaults(defineProps<{
    onStripeResponseCallback: (tokenId: unknown) => void,
}>(), {
    onStripeResponseCallback: () => console.error('onStripeResponse is not reinitialized'),
});

const isLoading = ref<boolean>(false);
/**
 * Stripe elements is using to create 'Add Card' form.
 */
const cardElement = ref<any>(); // eslint-disable-line @typescript-eslint/no-explicit-any
/**
 * Stripe library.
 */
const stripe = ref<any>(); // eslint-disable-line @typescript-eslint/no-explicit-any

/**
 * Stripe initialization.
 */
async function initStripe(): Promise<void> {
    const stripePublicKey = configStore.state.config.stripePublicKey;

    stripe.value = window['Stripe'](stripePublicKey);
    if (!stripe.value) {
        await notify.error('Unable to initialize stripe', AnalyticsErrorEventSource.BILLING_STRIPE_CARD_INPUT);
        return;
    }

    const elements = stripe.value.elements();
    if (!elements) {
        await notify.error('Unable to instantiate elements', AnalyticsErrorEventSource.BILLING_STRIPE_CARD_INPUT);
        return;
    }

    cardElement.value = elements.create('card');
    if (!cardElement.value) {
        await notify.error('Unable to create card', AnalyticsErrorEventSource.BILLING_STRIPE_CARD_INPUT);
        return;
    }

    cardElement.value.mount('#card-element');
    cardElement.value.addEventListener('change', function (event): void {
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
async function onStripeResponse(result: StripeResponse): Promise<void> {
    if (result.error) {
        throw result.error;
    }

    if (result.token.card.funding === 'prepaid') {
        notify.error('Prepaid cards are not supported', AnalyticsErrorEventSource.BILLING_STRIPE_CARD_INPUT);
        return;
    }

    await props.onStripeResponseCallback(result.token.id);
    cardElement.value.clear();
}

/**
 * Fires stripe event after all inputs are filled.
 */
async function onSubmit(): Promise<void> {
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
onMounted(async (): Promise<void> => {
    if (!window['Stripe']) {
        const script = new LoadScript('https://js.stripe.com/v3/',
            () => { initStripe(); },
            () => { notify.error('Stripe library not loaded', AnalyticsErrorEventSource.BILLING_STRIPE_CARD_INPUT);
                script.remove();
            },
        );

        return;
    }

    initStripe();
});

/**
 * Clears listeners.
 */
onBeforeUnmount(() => {
    cardElement.value?.removeEventListener('change');
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
        border: 1px solid transparent;
        border-radius: 4px;
        background-color: white;
        box-shadow: 0 1px 3px 0 #e6ebf1;
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
</style>

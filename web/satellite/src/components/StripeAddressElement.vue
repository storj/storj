// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-skeleton-loader type="card" :loading="isLoading">
        <div id="payment-element" class="pa-1 w-100">
        <!-- A Stripe Element will be inserted here. -->
        </div>
    </v-skeleton-loader>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { VSkeletonLoader } from 'vuetify/components';
import { loadStripe } from '@stripe/stripe-js/pure';
import {
    Stripe,
    StripeAddressElement,
    StripeElements,
    StripeAddressElementOptions,
    StripeElementsOptionsMode,
} from '@stripe/stripe-js';
import { useTheme } from 'vuetify';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { useConfigStore } from '@/store/modules/configStore';
import { BillingAddress } from '@/types/payments';
import { useLoading } from '@/composables/useLoading';

const configStore = useConfigStore();

const notify = useNotify();
const theme = useTheme();
const { isLoading, withLoading } = useLoading();

const props = defineProps<{
    initial: BillingAddress | null
}>();

const addressElement = ref<StripeAddressElement>();
const stripe = ref<Stripe | null>(null);
const elements = ref<StripeElements | null>(null);

async function initStripe(): Promise<void> {
    await withLoading(async () => {
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
        const addressOptions: StripeAddressElementOptions = {
            mode: 'shipping',
        };
        if (props.initial) {
            addressOptions.defaultValues =  {
                name: props.initial.name,
                address: {
                    line1: props.initial.line1,
                    line2: props.initial.line2,
                    city: props.initial.city,
                    state: props.initial.state,
                    postal_code: props.initial.postalCode,
                    country: props.initial.country.code,
                },
            };
        }
        addressElement.value = elements.value.create('address', addressOptions);
        if (!addressElement.value) {
            notify.error('Unable to create address element', AnalyticsErrorEventSource.BILLING_STRIPE_CARD_INPUT);
            return;
        }
    });
    addressElement.value?.mount('#payment-element');
}

/**
 * To be called by parent element to
 * validate and get filled form data.
 */
async function onSubmit(): Promise<BillingAddress> {
    if (!(stripe.value && elements.value && addressElement.value)) {
        throw new Error('Stripe is not initialized');
    }

    const { complete, value } = await addressElement.value.getValue();

    if (!complete) {
        throw new Error('Address is incomplete');
    }
    return {
        name: value.name,
        city: value.address.city,
        country: {
            code: value.address.country,
        },
        line1: value.address.line1,
        line2: value.address.line2,
        postalCode: value.address.postal_code,
        state: value.address.state,
    };
}

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
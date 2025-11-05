// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-expansion-panels class="mt-4">
        <v-expansion-panel eager static @group:selected="({value}) => onPanelToggle(value)">
            <v-expansion-panel-title>Billing info (optional)</v-expansion-panel-title>
            <v-expansion-panel-text>
                <v-progress-linear rounded indeterminate :active="isLoading" />
                <v-row>
                    <v-col>
                        <div id="billing-address-element">
                            <!-- A Stripe Address Element will be inserted here. -->
                        </div>
                    </v-col>
                    <v-col>
                        <template v-if="addressElementReady">
                            <v-select
                                v-model="selectedTax"
                                label="Tax ID type"
                                placeholder="Choose tax ID type"
                                :disabled="!countryCode"
                                :items="taxes"
                                :item-title="(item: Tax) => item.name"
                                :item-value="(item: Tax) => item"
                                hide-details
                                class="mb-3"
                            />

                            <v-text-field
                                v-model="taxID"
                                :disabled="!selectedTax"
                                variant="outlined"
                                label="Tax ID"
                                placeholder="Enter your Tax ID"
                                :hint="'e.g.: ' + selectedTax?.example"
                                :hide-details="false"
                                :maxlength="50"
                                class="custom"
                            />
                        </template>
                    </v-col>
                </v-row>
            </v-expansion-panel-text>
        </v-expansion-panel>
    </v-expansion-panels>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { loadStripe } from '@stripe/stripe-js/pure';
import {
    Stripe,
    StripeAddressElement,
    StripeElements,
    StripeElementsOptionsMode,
    StripeAddressElementChangeEvent,
} from '@stripe/stripe-js';
import { useTheme } from 'vuetify';
import {
    VRow,
    VCol,
    VProgressLinear,
    VExpansionPanels,
    VExpansionPanel,
    VExpansionPanelTitle,
    VExpansionPanelText,
    VSelect,
    VTextField,
} from 'vuetify/components';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { useConfigStore } from '@/store/modules/configStore';
import { useLoading } from '@/composables/useLoading';
import { PurchaseBillingInfo, Tax } from '@/types/payments';
import { useBillingStore } from '@/store/modules/billingStore';

const configStore = useConfigStore();
const billingStore = useBillingStore();

const notify = useNotify();
const theme = useTheme();
const { isLoading, withLoading } = useLoading();

const addressElement = ref<StripeAddressElement>();
const stripe = ref<Stripe | null>(null);
const elements = ref<StripeElements | null>(null);
const addressElementReady = ref(false);

const countryCode = ref<string>();
const selectedTax = ref<Tax>();
const taxID = ref<string>();

const taxes = computed<Tax[]>(() => billingStore.state.taxes);

function initStripe(): void {
    withLoading(async () => {
        const stripePublicKey = configStore.state.config.stripePublicKey;

        try {
            stripe.value = await loadStripe(stripePublicKey);
        } catch (error) {
            notify.error(error.message, AnalyticsErrorEventSource.BILLING_STRIPE_INFO_FORM);
            return;
        }

        if (!stripe.value) {
            notify.error('Unable to initialize stripe', AnalyticsErrorEventSource.BILLING_STRIPE_INFO_FORM);
            return;
        }

        const options: StripeElementsOptionsMode = {
            appearance: {
                theme: theme.global.current.value.dark ? 'night' : 'stripe',
                labels: 'floating',
            },
        };
        elements.value = stripe.value.elements(options);
        if (!elements.value) {
            notify.error('Unable to instantiate elements', AnalyticsErrorEventSource.BILLING_STRIPE_INFO_FORM);
        }
    });
}

function onPanelToggle(val: boolean): void {
    if (!val) {
        addressElement.value?.destroy();
        addressElementReady.value = false;
        taxID.value = undefined;
        return;
    }

    if (!elements.value) {
        notify.error('Unable to instantiate elements', AnalyticsErrorEventSource.BILLING_STRIPE_INFO_FORM);
        return;
    }

    addressElement.value = elements.value.create('address', { mode: 'billing' });
    addressElement.value.on('ready', () => {
        addressElementReady.value = true;
    });
    addressElement.value.on('change', (event: StripeAddressElementChangeEvent) => {
        if (countryCode.value !== event.value.address?.country) countryCode.value = event.value.address?.country;
    });
    addressElement.value.mount('#billing-address-element');
}

/**
 * To be called by parent element to
 * validate and get filled form data.
 */
async function onSubmit(): Promise<PurchaseBillingInfo> {
    if (!(stripe.value && elements.value)) {
        throw new Error('Stripe is not initialized');
    }

    if (!(addressElement.value && addressElementReady.value)) {
        return {
            address: undefined,
            tax: undefined,
        };
    }

    const { complete, value } = await addressElement.value.getValue();
    if (!complete) {
        throw new Error('Please fill out the form or skip it.');
    }

    return {
        address: complete ? {
            name: value.name,
            city: value.address.city,
            country: value.address.country,
            line1: value.address.line1,
            line2: value.address.line2,
            postalCode: value.address.postal_code,
            state: value.address.state,
        } : undefined,
        tax: selectedTax.value && taxID.value ? {
            type: selectedTax.value.code,
            value: taxID.value,
        } : undefined,
    };
}

watch(countryCode, (code) => {
    withLoading(async () => {
        if (!code) {
            return;
        }
        selectedTax.value = undefined;
        try {
            await billingStore.getCountryTaxes(code ?? '');
            if (taxes.value.length === 1) {
                selectedTax.value = taxes.value[0];
            }
        } catch (e) {
            notify.notifyError(e);
        }
    });
});

onMounted(() => {
    initStripe();
});

onBeforeUnmount(() => {
    addressElement.value?.off('change');
    addressElement.value?.off('ready');
});

defineExpose({
    onSubmit,
});
</script>

<style scoped lang="scss">
:deep(.v-field__input) {
    min-height: 64px;
}
</style>

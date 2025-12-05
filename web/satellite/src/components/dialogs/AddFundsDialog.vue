// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        scrollable
        max-width="500px"
        min-width="400px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card rounded="xlg">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <div v-if="step === Step.Success" class="mr-2 rounded border pa-1">
                            <v-icon color="success" size="24" :icon="CircleCheckBig" />
                        </div>
                        <v-card-title class="font-weight-bold">
                            {{ step === Step.Success ? 'Payment Successful' : 'Add Funds' }}
                        </v-card-title>
                    </template>

                    <template #append>
                        <v-btn
                            :icon="X"
                            variant="text"
                            size="small"
                            color="default"
                            @click="model = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-card-text>
                <v-window v-model="step" :touch="false">
                    <v-window-item :value="Step.EnterAmount">
                        <v-form v-model="formValid">
                            <p class="mb-4">Choose the amount you wish to deposit:</p>

                            <v-col class="mb-5">
                                <v-row class="ga-2">
                                    <v-chip
                                        v-for="value in [10, 25, 50, 100]"
                                        :key="value"
                                        :value="value"
                                        color="primary"
                                        variant="tonal"
                                        @click="amount = value"
                                    >
                                        ${{ value }}
                                    </v-chip>
                                </v-row>
                            </v-col>

                            <v-text-field
                                v-model="amount"
                                label="Amount to deposit"
                                prefix="$"
                                type="number"
                                :min="minAmount"
                                :max="maxAmount"
                                :rules="amountRules"
                                hide-details="auto"
                                required
                                class="mb-2"
                                @update:model-value="onUpdateAmount"
                            />

                            <v-alert v-if="isFreeTier" border class="mt-4" variant="outlined" title="Unlock Pro Features" color="info">
                                <p class="text-body-2">
                                    Adding at least $10 will automatically upgrade your account to Pro.
                                </p>
                            </v-alert>

                            <v-alert
                                v-if="isFrozenOrWarned"
                                border
                                class="mt-4"
                                variant="outlined"
                                title="Important!"
                                color="warning"
                            >
                                <div class="text-body-2">
                                    <p class="mb-2">
                                        <strong>The "Add Funds" feature cannot be applied to overdue invoices.</strong>
                                    </p>

                                    <p class="mb-1">How to pay an overdue invoice:</p>
                                    <ul class="pl-4">
                                        <li>
                                            <b>Pay directly from the invoice</b>:
                                            go to <a class="link" @click="navigateToBillingHistory">Billing History</a>,
                                            download your invoice, and click the <b>Pay online</b> link.
                                        </li>
                                        <li>
                                            <b>Add a payment method</b> for automatic payment:
                                            go to <a class="link" @click="navigateToPaymentMethods">Payment Methods</a>
                                            and add a card, which will be used to pay your overdue invoice.
                                        </li>
                                    </ul>
                                </div>
                            </v-alert>
                        </v-form>
                    </v-window-item>
                    <v-window-item :value="Step.ConfirmPayment">
                        <strong>Amount to be charged: ${{ amount }}</strong>

                        <p class="mt-4 mb-7">Select your payment method:</p>

                        <v-select
                            v-if="creditCards.length > 0 && !customCardForm"
                            v-model="selectedPaymentMethod"
                            label="Payment method"
                            :items="selectValues"
                            variant="outlined"
                            hide-details
                            class="mb-4"
                        >
                            <template #append-inner>
                                <v-chip v-if="isDefaultSelected" color="default" size="small" variant="tonal" class="font-weight-bold">Default</v-chip>
                            </template>
                            <template #item="{ props, item }">
                                <v-list-item v-bind="props">
                                    <template #append>
                                        <v-chip v-if="item.raw.isDefault" color="default" size="small" variant="tonal" class="font-weight-bold">Default</v-chip>
                                    </template>
                                </v-list-item>
                            </template>
                        </v-select>
                        <div id="express-checkout-element">
                            <!-- A Stripe Express Checkout Element will be inserted here. -->
                        </div>
                        <v-btn v-if="!customCardForm" variant="outlined" color="default" class="mt-2" block @click="activateCustomCardForm">
                            Use Another Card
                        </v-btn>
                        <div id="payment-element" class="mt-2">
                            <!-- A Stripe Payment Element will be inserted here. -->
                        </div>
                    </v-window-item>
                    <v-window-item :value="Step.Success">
                        <p>
                            Your payment of ${{ amount.toFixed(2) }} has been processed successfully.
                            Funds will be added to your account shortly.
                        </p>
                    </v-window-item>
                </v-window>
            </v-card-text>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col v-if="step === Step.EnterAmount">
                        <v-btn variant="outlined" color="default" block @click="model = false">
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="primary" variant="flat" block :loading="isLoading" :disabled="!formValid" @click="proceed">
                            {{ nextButtonLabel }}
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import {
    VAlert,
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardText,
    VCardTitle,
    VChip,
    VCol,
    VDialog,
    VDivider,
    VForm,
    VIcon,
    VListItem,
    VRow,
    VSelect,
    VSheet,
    VTextField,
    VWindow,
    VWindowItem,
} from 'vuetify/components';
import { computed, nextTick, onMounted, ref, watch } from 'vue';
import { loadStripe } from '@stripe/stripe-js/pure';
import { Stripe, StripeElements, StripeElementsOptionsMode } from '@stripe/stripe-js';
import { CircleCheckBig, X } from 'lucide-vue-next';
import { useTheme } from 'vuetify';
import { useRouter } from 'vue-router';

import { useLoading } from '@/composables/useLoading';
import { ChargeCardIntent, CreditCard } from '@/types/payments';
import { useBillingStore } from '@/store/modules/billingStore';
import { RequiredRule, ValidationRule } from '@/types/common';
import { useConfigStore } from '@/store/modules/configStore';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useUsersStore } from '@/store/modules/usersStore';
import { ROUTES } from '@/router';

type SelectValue = {
    title: string;
    value: string;
    isDefault: boolean;
};

enum Step {
    EnterAmount,
    ConfirmPayment,
    Success,
}

const configStore = useConfigStore();
const billingStore = useBillingStore();
const userStore = useUsersStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();
const theme = useTheme();
const router = useRouter();

const model = defineModel<boolean>({ required: true });

const step = ref<Step>(Step.EnterAmount);
const formValid = ref<boolean>(false);
const selectedPaymentMethod = ref<string>();
const amount = ref<number>(10);
const isDefaultSelected = ref<boolean>(true);
const stripe = ref<Stripe | null>(null);
const customCardForm = ref<boolean>(false);
const customCardFormElements = ref<StripeElements>();

const isFreeTier = computed<boolean>(() => userStore.state.user.isFree);

const isFrozenOrWarned = computed<boolean>(() => userStore.state.user.freezeStatus.frozen || userStore.state.user.freezeStatus.warned);

const amountRules = computed<ValidationRule<string>[]>(() => {
    return [
        RequiredRule,
        v => !(isNaN(+v) || isNaN(parseInt(v))) || 'Invalid number',
        v => !/[.,]/.test(v) || 'Value must be a whole number',
        v => (parseInt(v) > 0) || 'Value must be a positive number',
        v => {
            if (parseInt(v) > maxAmount.value) return `Amount must be less than or equal to ${maxAmount.value}`;
            if (parseInt(v) < minAmount.value) return `Amount must be more than or equal to ${minAmount.value}`;
            return true;
        },
    ];
});

const minAmount = computed<number>(() => configStore.state.config.minAddFundsAmount / 100);
const maxAmount = computed<number>(() => configStore.state.config.maxAddFundsAmount / 100);

const creditCards = computed((): CreditCard[] => billingStore.state.creditCards);

const selectValues = computed<SelectValue[]>(() => creditCards.value.map(card => {
    return { title: `${card.brand} **** ${card.last4}`, value: card.id, isDefault: card.isDefault };
}));

const nextButtonLabel = computed<string>(() => {
    if (step.value === Step.Success) return 'Done';
    if (step.value === Step.ConfirmPayment && customCardForm.value) return `Pay $${amount.value}`;
    return 'Continue';
});

function navigateToBillingHistory(): void {
    model.value = false;
    router.push(`${ROUTES.Billing.path}?tab=billing-history`);
}

function navigateToPaymentMethods(): void {
    model.value = false;
    router.push(`${ROUTES.Billing.path}?tab=payment-methods`);
}

function onUpdateAmount(value: string): void {
    if (!value) {
        amount.value = 1;
        return;
    }

    const num = +value;
    if (isNaN(num) || isNaN(parseInt(value))) return;
    amount.value = num;
}

function proceed(): void {
    if (step.value === Step.Success) {
        model.value = false;
        return;
    }
    if (step.value === Step.EnterAmount) {
        if (!formValid.value) return;

        step.value = Step.ConfirmPayment;
        return;
    }

    withLoading(async () => {
        try {
            if (customCardForm.value) {
                // Custom card flow.
                await handleCustomCardPaymentConfirmation();
            } else {
                // Regular existing card flow.
                if (!selectedPaymentMethod.value) {
                    notify.error('Payment method must be selected', AnalyticsErrorEventSource.ADD_FUNDS_DIALOG);
                    return;
                }

                const resp = await billingStore.addFunds(selectedPaymentMethod.value, amount.value * 100, ChargeCardIntent.AddFunds);
                if (resp.success) {
                    notify.success('Payment confirmed! Your account balance will be updated shortly.');
                    step.value = Step.Success;
                } else if (resp.paymentIntentID && resp.clientSecret) {
                    await handlePaymentConfirmation(resp.clientSecret);
                } else {
                    notify.error('Failed to add funds', AnalyticsErrorEventSource.ADD_FUNDS_DIALOG);
                }
            }
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.ADD_FUNDS_DIALOG);
        }
    });
}

async function handlePaymentConfirmation(clientSecret: string): Promise<void> {
    if (!stripe.value) {
        notify.error('Stripe failed to initialize.', AnalyticsErrorEventSource.ADD_FUNDS_DIALOG);
        return;
    }

    const { error, paymentIntent } = await stripe.value.confirmCardPayment(
        clientSecret,
        { payment_method: selectedPaymentMethod.value },
    );

    if (error) {
        notify.error(error.message ?? 'Payment confirmation failed.', AnalyticsErrorEventSource.ADD_FUNDS_DIALOG);
        return;
    }
    if (paymentIntent?.status !== 'succeeded') {
        notify.warning('Payment confirmation failed.');
    } else {
        notify.success('Payment confirmed! Your account balance will be updated shortly.');
        step.value = Step.Success;
    }
}

async function handleCustomCardPaymentConfirmation(): Promise<void> {
    if (!customCardFormElements.value) {
        notify.error('Form elements failed to render', AnalyticsErrorEventSource.ADD_FUNDS_DIALOG);
        return;
    }

    const { error } = await customCardFormElements.value.submit();
    if (error) {
        notify.error(error.message ?? 'Payment confirmation failed.', AnalyticsErrorEventSource.ADD_FUNDS_DIALOG);
        return;
    }

    await confirmPayment(customCardFormElements.value, true);
}

async function confirmPayment(elements: StripeElements, withCustomCard: boolean): Promise<void> {
    if (!stripe.value) {
        notify.error('Stripe failed to initialize.', AnalyticsErrorEventSource.ADD_FUNDS_DIALOG);
        return;
    }

    const clientSecret = await billingStore.createIntent(amount.value * 100, withCustomCard);

    const { error: confirmErr, paymentIntent } = await stripe.value.confirmPayment({
        elements,
        clientSecret,
        confirmParams: { return_url: `${window.location.origin}/account/billing` },
        redirect: 'if_required',
    });
    if (confirmErr) {
        notify.error(confirmErr.message ?? 'Payment confirmation failed.', AnalyticsErrorEventSource.ADD_FUNDS_DIALOG);
        return;
    }
    if (paymentIntent?.status !== 'succeeded') {
        notify.warning('Payment confirmation failed.');
    } else {
        notify.success('Payment confirmed! Your account balance will be updated shortly.');
        step.value = Step.Success;
    }
}

// Activates the custom card form and initializes Stripe Elements.
function activateCustomCardForm(): void {
    withLoading(async () => {
        try {
            if (!stripe.value) {
                notify.error('Stripe failed to initialize.', AnalyticsErrorEventSource.ADD_FUNDS_DIALOG);
                return;
            }

            customCardForm.value = true;

            const options: StripeElementsOptionsMode = {
                appearance: {
                    theme: theme.global.current.value.dark ? 'night' : 'stripe',
                    labels: 'floating',
                },
                payment_method_types: ['card'],
                mode: 'payment',
                amount: amount.value * 100,
                currency: 'usd',
            };
            customCardFormElements.value = stripe.value.elements(options);
            const paymentElement = customCardFormElements.value.create('payment', {
                wallets: {
                    applePay: 'never',
                    googlePay: 'never',
                },
            });
            paymentElement.mount('#payment-element');
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.ADD_FUNDS_DIALOG);
        }
    });
}

watch(model, async newVal => {
    if (!newVal) {
        if (step.value === Step.Success) {
            Promise.all([
                billingStore.getBalance(),
                userStore.getUser(),
            ]).catch(() => {});
        }

        amount.value = 10;
        formValid.value = false;
        step.value = Step.EnterAmount;
        customCardForm.value = false;
    }

    selectedPaymentMethod.value = creditCards.value.find(card => card.isDefault)?.id;
});

watch(selectedPaymentMethod, newVal => {
    isDefaultSelected.value = creditCards.value.some(
        card => card.isDefault && card.id === newVal,
    );
});

watch(step, newStep => {
    if (newStep !== Step.ConfirmPayment) return;

    if (!stripe.value) {
        notify.error('Stripe failed to initialize.', AnalyticsErrorEventSource.ADD_FUNDS_DIALOG);
        return;
    }

    const options: StripeElementsOptionsMode = {
        appearance: {
            theme: theme.global.current.value.dark ? 'night' : 'stripe',
            labels: 'floating',
        },
        mode: 'payment',
        amount: amount.value * 100,
        currency: 'usd',
    };
    const elements = stripe.value.elements(options);

    const expressCheckout = elements.create('expressCheckout', {
        paymentMethodOrder: ['googlePay', 'applePay', 'amazonPay', 'link', 'klarna'],
        layout: {
            maxColumns: 1,
            overflow: 'never',
        },
    });
    expressCheckout.on('confirm', () => {
        withLoading(async () => {
            try {
                await confirmPayment(elements, false);
            } catch (error) {
                notify.notifyError(error, AnalyticsErrorEventSource.ADD_FUNDS_DIALOG);
            }
        });
    });

    nextTick(() => {
        expressCheckout.mount('#express-checkout-element');
    });
});

onMounted(async () => {
    if (!stripe.value) stripe.value = await loadStripe(configStore.state.config.stripePublicKey);
});
</script>

<style scoped lang="scss">
.v-overlay .v-card .v-window {
    overflow-y: hidden !important;
}
</style>

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="flat" title="Add Card">
        <v-card-text>
            <v-btn v-if="!isCardInputShown" variant="outlined" color="default" size="small" rounded="md" class="mr-2" @click="onShowCardInput">+ Add New Card</v-btn>

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
    <v-dialog
        v-model="isUpgradeSuccessShown"
        width="460px"
        transition="fade-transition"
    >
        <v-card>
            <v-card-item class="pa-6">
                <template #prepend>
                    <img class="d-block" src="@/assets/icon-success.svg" alt="success">
                </template>

                <v-card-title class="font-weight-bold">Success</v-card-title>

                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        @click="isUpgradeSuccessShown = false"
                    />
                </template>
            </v-card-item>

            <v-card-item class="py-4">
                <SuccessStep @continue="isUpgradeSuccessShown = false" />
            </v-card-item>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { VBtn, VCard, VCardItem, VCardText, VCardTitle, VDialog } from 'vuetify/components';
import { computed, ref } from 'vue';

import { useUsersStore } from '@/store/modules/usersStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { useBillingStore } from '@/store/modules/billingStore';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useAppStore } from '@/store/modules/appStore';

import StripeCardElement from '@/components/StripeCardElement.vue';
import StripeCardInput from '@/components/StripeCardInput.vue';
import SuccessStep from '@/components/dialogs/upgradeAccountFlow/SuccessStep.vue';

interface StripeForm {
    onSubmit(): Promise<string>;
}

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const usersStore = useUsersStore();
const billingStore = useBillingStore();
const appStore = useAppStore();

const notify = useNotify();
const { withLoading, isLoading } = useLoading();

const stripeCardInput = ref<StripeForm | null>(null);

const isCardInputShown = ref(false);
const isUpgradeSuccessShown = ref(false);

/**
 * Indicates whether stripe payment element is enabled.
 */
const paymentElementEnabled = computed(() => {
    return configStore.state.config.stripePaymentElementEnabled;
});

/**
 * Triggers enter card info inputs to be shown.
 */
function onShowCardInput(): void {
    if (!usersStore.state.user.paidTier) {
        appStore.toggleUpgradeFlow(true);
        return;
    }

    isCardInputShown.value = true;
}

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

        billingStore.getCreditCards().catch((_) => {});
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
        return;
    }

    const frozenOrWarned = usersStore.state.user.freezeStatus?.frozen ||
            usersStore.state.user.freezeStatus?.trialExpiredFrozen ||
            usersStore.state.user.freezeStatus?.warned;
    if (frozenOrWarned) {
        try {
            await billingStore.attemptPayments();
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
        }
    }
    const oldPaidTier = usersStore.state.user.paidTier;
    if (oldPaidTier && !frozenOrWarned) {
        return;
    }
    try {
    // We fetch User one more time to update their Paid Tier and freeze status.
        await usersStore.getUser();
        const newPaidTier = usersStore.state.user.paidTier;
        if (!oldPaidTier && newPaidTier) {
            isUpgradeSuccessShown.value = true;
        }
    } catch { /* empty */ }
}
</script>

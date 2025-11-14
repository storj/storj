// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card title="Add New Card" subtitle="Add a new credit/debit card for payment." class="pa-2">
        <v-card-text>
            <div v-if="!isCardInputShown">
                <v-btn variant="outlined" color="default" class="mr-2" :prepend-icon="Plus" @click="onShowCardInput">Add New Card</v-btn>
            </div>

            <StripeCardElement
                v-else
                ref="stripeCardInput"
                @ready="stripeReady = true"
            />

            <div v-if="isCardInputShown" class="mt-4">
                <v-btn
                    color="primary"
                    class="mr-2"
                    :loading="isLoading"
                    :disabled="!stripeReady"
                    @click="onSaveCardClick"
                >
                    Add Card
                </v-btn>
                <v-btn
                    variant="outlined"
                    color="default"
                    class="mr-2"
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
                        :icon="X"
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
import { ref, watch } from 'vue';
import { Plus, X } from 'lucide-vue-next';

import { useUsersStore } from '@/store/modules/usersStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { useBillingStore } from '@/store/modules/billingStore';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useAppStore } from '@/store/modules/appStore';
import { StripeForm } from '@/types/common';

import StripeCardElement from '@/components/StripeCardElement.vue';
import SuccessStep from '@/components/dialogs/upgradeAccountFlow/SuccessStep.vue';

const analyticsStore = useAnalyticsStore();
const usersStore = useUsersStore();
const billingStore = useBillingStore();
const appStore = useAppStore();

const notify = useNotify();
const { withLoading, isLoading } = useLoading();

const stripeCardInput = ref<StripeForm | null>(null);
const stripeReady = ref<boolean>(false);

const isCardInputShown = ref(false);
const isUpgradeSuccessShown = ref(false);

/**
 * Triggers enter card info inputs to be shown.
 */
function onShowCardInput(): void {
    if (usersStore.state.user.isFree) {
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
 * @param res - the response from stripe.
 * depending on the paymentElementEnabled flag.
 */
async function addCardToDB(res: string) {
    try {
        await billingStore.addCardByPaymentMethodID({ token: res });
        notify.success('Card successfully added');
        isCardInputShown.value = false;

        analyticsStore.eventTriggered(AnalyticsEvent.CREDIT_CARD_ADDED_FROM_BILLING);

        billingStore.getCreditCards().catch((_) => {});
    } catch (error) {
        // initStripe will get a new card setup secret if there's an error
        // on our side after the card is already added on stripe side.
        stripeCardInput.value?.initStripe();

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
    const oldPaidTier = usersStore.state.user.isPaid;
    if (oldPaidTier && !frozenOrWarned) {
        return;
    }
    try {
        // We fetch User one more time to update their Paid Tier and freeze status.
        await usersStore.getUser();
        const newPaidTier = usersStore.state.user.isPaid;
        if (!oldPaidTier && newPaidTier) {
            isUpgradeSuccessShown.value = true;
        }
    } catch { /* empty */ }
}

watch(isCardInputShown, (shown) => {
    if (!shown) stripeReady.value = false;
});
</script>

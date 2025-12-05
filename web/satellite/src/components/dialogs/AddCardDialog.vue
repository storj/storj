// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        :persistent="isLoading"
        max-width="500px"
        transition="fade-transition"
        :scrim
    >
        <v-card>
            <v-card-item class="pa-5 pl-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <icon-card />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">Add New Card</v-card-title>
                <template #append>
                    <v-btn
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-card-text>
                <StripeCardElement
                    ref="stripeCardInput"
                    @ready="stripeReady = true"
                />
            </v-card-text>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-row>
                        <v-col>
                            <v-btn variant="outlined" color="default" block :disabled="isLoading" @click="model = false">
                                Cancel
                            </v-btn>
                        </v-col>
                        <v-col>
                            <v-btn
                                color="primary" variant="flat" block
                                :disabled="!stripeReady"
                                :loading="isLoading" @click="onSaveCardClick"
                            >
                                Add Card
                            </v-btn>
                        </v-col>
                    </v-row>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardText,
    VCardTitle,
    VCol,
    VDialog,
    VDivider,
    VRow,
    VSheet,
} from 'vuetify/components';
import { X } from 'lucide-vue-next';

import { useBillingStore } from '@/store/modules/billingStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useUsersStore } from '@/store/modules/usersStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { StripeForm } from '@/types/common';

import IconCard from '@/components/icons/IconCard.vue';
import StripeCardElement from '@/components/StripeCardElement.vue';

const analyticsStore = useAnalyticsStore();
const billingStore = useBillingStore();

const usersStore = useUsersStore();
const { isLoading, withLoading } = useLoading();

const notify = useNotify();

defineProps<{
    scrim: boolean,
}>();

const model = defineModel<boolean>({ required: true });

const stripeCardInput = ref<StripeForm | null>(null);
const stripeReady = ref<boolean>(false);

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

        analyticsStore.eventTriggered(AnalyticsEvent.CREDIT_CARD_ADDED_FROM_BILLING);
    } catch (error) {
        // initStripe will get a new card setup secret if there's an error
        // on our side after the card is already added on stripe side.
        stripeCardInput.value?.initStripe();

        notify.notifyError(error, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
        return;
    }

    try {
        await billingStore.getCreditCards();
        // eslint-disable-next-line @typescript-eslint/no-unused-vars
    } catch (_) {
        // ignore
    } finally {
        model.value = false;
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
}

watch(model, (shown) => {
    if (!shown) stripeReady.value = false;
});
</script>

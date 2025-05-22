// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row class="ma-0" align="center">
        <v-col class="px-0">
            <p class="font-weight-bold">{{ plan.title }} <span v-if="plan.activationSubtitle"> / {{ plan.activationSubtitle }}</span></p>
            <!-- eslint-disable-next-line vue/no-v-html -->
            <p v-html="plan.activationDescriptionHTML" />
            <!-- eslint-disable-next-line vue/no-v-html -->
            <v-chip v-if="plan.activationPriceHTML" color="success" :prepend-icon="Check" class="mt-2"><p class="font-weight-bold" v-html="plan.activationPriceHTML" /></v-chip>
        </v-col>
    </v-row>

    <p class="text-caption mb-4">Add Card Info</p>

    <StripeCardElement
        ref="stripeCardInput"
        @ready="stripeReady = true"
    />

    <v-row justify="center" class="mx-0 mt-2 mb-1">
        <v-col class="pl-0">
            <v-btn
                block
                variant="outlined"
                color="default"
                :disabled="loading"
                @click="emit('back')"
            >
                Back
            </v-btn>
        </v-col>
        <v-col class="px-0">
            <v-btn
                block
                :loading="loading"
                :disabled="!stripeReady"
                :prepend-icon="LockKeyhole"
                @click="onSaveCardClick"
            >
                Save card
            </v-btn>
        </v-col>
    </v-row>
    <p class="text-caption text-center">Information is secured with 128-bit SSL & AES-256 encryption.</p>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import { useRoute } from 'vue-router';
import { VBtn, VChip, VCol, VRow } from 'vuetify/components';
import { Check, LockKeyhole } from 'lucide-vue-next';

import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { useBillingStore } from '@/store/modules/billingStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { ROUTES } from '@/router';

import StripeCardElement from '@/components/StripeCardElement.vue';

interface StripeForm {
    onSubmit(): Promise<string>;
}

const analyticsStore = useAnalyticsStore();
const usersStore = useUsersStore();
const billingStore = useBillingStore();
const projectsStore = useProjectsStore();

const notify = useNotify();
const route = useRoute();

const plan = computed(() => billingStore.proPlanInfo);

const loading = defineModel<boolean>('loading', { default: false });

const emit = defineEmits<{
    success: [];
    back: [];
}>();

const stripeCardInput = ref<StripeForm | null>(null);
const stripeReady = ref<boolean>(false);

/**
 * Provides card information to Stripe.
 */
async function onSaveCardClick(): Promise<void> {
    if (loading.value || !stripeCardInput.value) return;

    loading.value = true;
    try {
        const response = await stripeCardInput.value.onSubmit();
        await addCardToDB(response);
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.UPGRADE_ACCOUNT_MODAL);
    }
    loading.value = false;
}

/**
 * Adds card after Stripe confirmation.
 *
 * @param res - the response from stripe. Could be a token or a payment method id.
 * depending on the paymentElementEnabled flag.
 */
async function addCardToDB(res: string): Promise<void> {
    try {
        await billingStore.addCardByPaymentMethodID(res);
        notify.success('Card successfully added');
        // We fetch User one more time to update their Paid Tier status.
        usersStore.getUser().catch((_) => {});

        if (route.name === ROUTES.Dashboard.name) {
            projectsStore.getProjectConfig().catch((_) => {});
            projectsStore.getProjectLimits(projectsStore.state.selectedProject.id).catch((_) => {});
        }

        if (route.name === ROUTES.Billing.name) {
            billingStore.getCreditCards().catch((_) => {});
        }

        analyticsStore.eventTriggered(AnalyticsEvent.MODAL_ADD_CARD);

        emit('success');
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.UPGRADE_ACCOUNT_MODAL);
    }
}
</script>

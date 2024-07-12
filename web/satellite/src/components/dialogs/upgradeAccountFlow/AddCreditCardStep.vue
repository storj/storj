// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <p class="mb-4">
        By saving your card information, you allow Storj to charge your card for future payments in accordance with
        the <a class="link font-weight-medium" href="https://storj.io/terms-of-service/" target="_blank" rel="noopener">terms of service</a>.
    </p>

    <StripeCardElement
        v-if="paymentElementEnabled"
        ref="stripeCardInput"
    />
    <StripeCardInput
        v-else
        ref="stripeCardInput"
    />

    <v-row justify="center" class="mx-0 mt-4 mb-1">
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
                color="success"
                :loading="loading"
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
import { computed, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { VBtn, VCol, VRow } from 'vuetify/components';
import { LockKeyhole } from 'lucide-vue-next';

import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useBillingStore } from '@/store/modules/billingStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { ROUTES } from '@/router';

import StripeCardElement from '@/components/StripeCardElement.vue';
import StripeCardInput from '@/components/StripeCardInput.vue';

interface StripeForm {
    onSubmit(): Promise<string>;
}

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const usersStore = useUsersStore();
const billingStore = useBillingStore();
const projectsStore = useProjectsStore();

const notify = useNotify();
const router = useRouter();
const route = useRoute();

const loading = defineModel<boolean>('loading', { default: false });

const emit = defineEmits<{
    success: [];
    back: [];
}>();

const stripeCardInput = ref<StripeForm | null>(null);

/**
 * Indicates whether stripe payment element is enabled.
 */
const paymentElementEnabled = computed(() => {
    return configStore.state.config.stripePaymentElementEnabled;
});

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
        const action = paymentElementEnabled.value ? billingStore.addCardByPaymentMethodID : billingStore.addCreditCard;
        await action(res);
        notify.success('Card successfully added');
        // We fetch User one more time to update their Paid Tier status.
        usersStore.getUser().catch((_) => {});

        if (route.name === ROUTES.Dashboard.name) {
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

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <p class="pt-2 pb-4">
        By saving your card information, you allow Storj to charge your card for future payments in accordance with
        the terms.
    </p>

    <div class="py-4">
        <StripeCardElement
            v-if="paymentElementEnabled"
            ref="stripeCardInput"
            :is-dark-theme="theme.global.current.value.dark"
            @pm-created="addCardToDB"
        />
        <StripeCardInput
            v-else
            ref="stripeCardInput"
            :on-stripe-response-callback="addCardToDB"
        />
    </div>

    <div class="pt-4">
        <v-row justify="center" class="mx-0 pb-3">
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
                    @click="onSaveCardClick"
                >
                    <template #prepend>
                        <v-icon icon="mdi-lock" />
                    </template>
                    Save card
                </v-btn>
            </v-col>
        </v-row>
        <p class="mt-1 text-caption text-center">Information is secured with 128-bit SSL & AES-256 encryption.</p>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { VBtn, VIcon, VCol, VRow } from 'vuetify/components';
import { useTheme } from 'vuetify';

import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { RouteConfig } from '@/types/router';
import { useNotify } from '@/utils/hooks';
import { useBillingStore } from '@/store/modules/billingStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useConfigStore } from '@/store/modules/configStore';

import StripeCardElement from '@/components/account/billing/paymentMethods/StripeCardElement.vue';
import StripeCardInput from '@/components/account/billing/paymentMethods/StripeCardInput.vue';

interface StripeForm {
    onSubmit(): Promise<void>;
}

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const usersStore = useUsersStore();
const billingStore = useBillingStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const theme = useTheme();
const router = useRouter();
const route = useRoute();

const emit = defineEmits<{
  success: [];
  back: [];
}>();

const loading = ref<boolean>(false);
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
        await stripeCardInput.value.onSubmit();
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
    loading.value = true;
    try {
        const action = paymentElementEnabled.value ? billingStore.addCardByPaymentMethodID : billingStore.addCreditCard;
        await action(res);
        notify.success('Card successfully added');
        // We fetch User one more time to update their Paid Tier status.
        await usersStore.getUser();

        if (route.path.includes(RouteConfig.ProjectDashboard.name.toLowerCase())) {
            await projectsStore.getProjectLimits(projectsStore.state.selectedProject.id);
        }

        if (route.path.includes(RouteConfig.Billing.path)) {
            await billingStore.getCreditCards();
        }

        analyticsStore.eventTriggered(AnalyticsEvent.MODAL_ADD_CARD);

        emit('success');
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.UPGRADE_ACCOUNT_MODAL);
    }

    loading.value = false;
}
</script>
// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <p class="pb-4">
        By saving your card information, you allow Storj to charge your card for future payments in accordance with
        the terms.
    </p>

    <v-divider />

    <div class="py-4">
        <StripeCardElement
            ref="stripeCardInput"
            :is-dark-theme="theme.global.current.value.dark"
            @pm-created="addCardToDB"
        />
    </div>

    <div class="pt-4">
        <v-row justify="center" class="mx-0 pb-3">
            <v-col class="pl-0">
                <v-btn
                    block
                    variant="outlined"
                    color="grey-lighten-1"
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
        <p class="mt-1 text-caption text-center">Your information is secured with 128-bit SSL & AES-256 encryption.</p>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { VDivider, VBtn, VIcon, VCol, VRow } from 'vuetify/components';
import { useTheme } from 'vuetify';

import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { RouteConfig } from '@/types/router';
import { useNotify } from '@/utils/hooks';
import { useBillingStore } from '@/store/modules/billingStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import StripeCardElement from '@/components/account/billing/paymentMethods/StripeCardElement.vue';

interface StripeForm {
    onSubmit(): Promise<void>;
}

const analyticsStore = useAnalyticsStore();
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
 * @param pmID - payment method ID from Stripe
 */
async function addCardToDB(pmID: string): Promise<void> {
    loading.value = true;
    try {
        await billingStore.addCardByPaymentMethodID(pmID);
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
// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="410px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card>
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <component :is="TicketPercent" :size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">Apply New Coupon</v-card-title>
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

            <v-card-item class="px-6 pt-6 pb-2">
                <p>If you have a coupon active, it will automatically be replaced.</p>
            </v-card-item>

            <v-card-item class="px-6 pb-4">
                <v-form v-model="formValid" @submit.prevent="onApplyClick">
                    <v-text-field
                        v-model="couponCode"
                        variant="outlined"
                        :rules="[RequiredRule]"
                        label="Coupon Code"
                        :hide-details="false"
                        maxlength="50"
                        class="pt-2"
                        autofocus
                    />
                </v-form>
            </v-card-item>
            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            :disabled="isLoading"
                            variant="outlined"
                            color="default"
                            block
                            @click="model = false"
                        >
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="secondary"
                            variant="flat"
                            block
                            :loading="isLoading"
                            @click="onApplyClick"
                        >
                            Activate Coupon
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import { VDialog, VCard, VRow, VCol, VTextField, VForm, VBtn, VCardItem, VCardTitle, VDivider, VCardActions, VSheet } from 'vuetify/components';
import { TicketPercent, X } from 'lucide-vue-next';

import { RequiredRule } from '@/types/common';
import { useLoading } from '@/composables/useLoading';
import { useBillingStore } from '@/store/modules/billingStore';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

const billingStore = useBillingStore();
const analyticsStore = useAnalyticsStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const model = defineModel<boolean>({ required: true });

const formValid = ref<boolean>(false);
const couponCode = ref<string>('');

/**
 * Applies the input coupon code or prompts the user if a coupon is already applied.
 */
async function onApplyClick(): Promise<void> {
    if (!formValid.value) return;

    await withLoading(async () => {
        try {
            await billingStore.applyCouponCode(couponCode.value);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.BILLING_APPLY_COUPON_CODE_INPUT);
            return;
        }
        analyticsStore.eventTriggered(AnalyticsEvent.COUPON_CODE_APPLIED);
        notify.success('Coupon applied.');
        model.value = false;
    });
}

watch(model, () => {
    couponCode.value = '';
});
</script>

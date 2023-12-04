// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="400px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card rounded="xlg" class="pa-7 pt-12">
            <v-btn
                icon="$close"
                variant="text"
                color="default"
                position="absolute"
                location="top right"
                class="mt-1 mr-1"
                @disabled="isLoading"
                @click="model = false"
            />

            <div cols="12" class="d-flex pb-6 justify-center">
                <img src="@/../static/images/account/billing/greenCoupon.svg" alt="Coupon">
            </div>

            <v-row>
                <v-col cols="12" class="text-center">
                    <p class="text-h5 font-weight-black pb-4">Apply New Coupon</p>
                    If you have a coupon active, it will automatically be replaced.
                </v-col>

                <v-col cols="12">
                    <v-form v-model="formValid" @submit.prevent="onApplyClick">
                        <v-text-field
                            v-model="couponCode"
                            variant="outlined"
                            :rules="[RequiredRule]"
                            label="Coupon Code"
                            :hide-details="false"
                            maxlength="50"
                            autofocus
                        />
                    </v-form>
                </v-col>

                <v-col cols="12" class="text-center">
                    <v-btn
                        color="primary"
                        variant="flat"
                        :loading="isLoading"
                        @click="onApplyClick"
                    >
                        Activate Coupon
                    </v-btn>
                </v-col>
            </v-row>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { VDialog, VCard, VRow, VCol, VTextField, VForm, VBtn } from 'vuetify/components';

import { RequiredRule } from '@poc/types/common';
import { useLoading } from '@/composables/useLoading';
import { useBillingStore } from '@/store/modules/billingStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

const billingStore = useBillingStore();
const analyticsStore = useAnalyticsStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const emit = defineEmits<{
    'update:modelValue': [boolean];
}>();

const props = defineProps<{
    modelValue: boolean;
}>();

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

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

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="400px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card ref="innerContent" rounded="xlg" class="pa-7 pt-12">
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

            <v-window v-model="step">
                <v-window-item :value="ApplyCouponCodeStep.Apply">
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
                </v-window-item>

                <v-window-item :value="ApplyCouponCodeStep.Confirm">
                    <v-row>
                        <v-col cols="12" class="text-center">
                            <p class="text-h5 font-weight-black pb-4">Replace Coupon?</p>
                            Your current coupon
                            <span class="font-weight-bold">{{ currentCoupon }}</span>
                            will be replaced with the new one.
                        </v-col>
                    </v-row>

                    <v-row justify="center">
                        <v-col cols="auto">
                            <v-btn color="default" variant="outlined" :disabled="isLoading" @click="model = false">
                                Cancel
                            </v-btn>
                        </v-col>

                        <v-col cols="auto">
                            <v-btn color="primary" variant="flat" :loading="isLoading" @click="onApplyClick">
                                Confirm
                            </v-btn>
                        </v-col>
                    </v-row>
                </v-window-item>
            </v-window>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref, computed, Component, watch } from 'vue';
import { VDialog, VCard, VWindow, VWindowItem, VRow, VCol, VTextField, VForm, VBtn } from 'vuetify/components';

import { RequiredRule } from '@poc/types/common';
import { useLoading } from '@/composables/useLoading';
import { useBillingStore } from '@/store/modules/billingStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

enum ApplyCouponCodeStep {
    Apply,
    Confirm,
}

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

const innerContent = ref<Component | null>(null);
const step = ref<ApplyCouponCodeStep>(ApplyCouponCodeStep.Apply);
const formValid = ref<boolean>(false);
const couponCode = ref<string>('');
const currentCoupon = ref<string>('');

/**
 * Applies the input coupon code or prompts the user if a coupon is already applied.
 */
async function onApplyClick(): Promise<void> {
    if (!formValid.value) return;

    if (step.value === ApplyCouponCodeStep.Apply && currentCoupon.value) {
        step.value = ApplyCouponCodeStep.Confirm;
        return;
    }

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

watch(innerContent, comp => {
    if (comp) {
        currentCoupon.value = billingStore.state.coupon?.name || '';
        return;
    }
    step.value = ApplyCouponCodeStep.Apply;
    couponCode.value = '';
});
</script>

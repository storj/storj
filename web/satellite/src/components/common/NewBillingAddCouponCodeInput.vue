// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-coupon__input-container">
        <div class="add-coupon__input-wrapper">
            <VInput
                placeholder="Enter Coupon Code"
                @setData="setCouponCode"
            />
        </div>
        <ValidationMessage
            class="add-coupon__valid-message"
            success-message="Successfully applied coupon code."
            :error-message="errorMessage"
            :is-valid="isCodeValid"
            :show-message="showValidationMessage"
        />
        <VButton
            class="add-coupon__apply-button"
            label="Apply Coupon Code"
            width="100%"
            height="44px"
            font-size="14px"
            :on-press="applyCouponCode"
            :is-disabled="isLoading"
        />
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useLoading } from '@/composables/useLoading';
import { useBillingStore } from '@/store/modules/billingStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import VInput from '@/components/common/VInput.vue';
import ValidationMessage from '@/components/common/ValidationMessage.vue';
import VButton from '@/components/common/VButton.vue';

const analyticsStore = useAnalyticsStore();
const billingStore = useBillingStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const emit = defineEmits(['close']);

const showValidationMessage = ref<boolean>(false);
const isCodeValid = ref<boolean>(false);
const errorMessage = ref<string>('');
const couponCode = ref<string>('');

function setCouponCode(value: string): void {
    couponCode.value = value;
}

/**
 * Check if coupon code is valid
 */
async function applyCouponCode(): Promise<void> {
    await withLoading(async () => {
        try {
            await billingStore.applyCouponCode(couponCode.value);
            notify.success('Coupon Added!');
            emit('close');
        } catch (error) {
            errorMessage.value = error.message;
            isCodeValid.value = false;
            showValidationMessage.value = true;
            analyticsStore.errorEventTriggered(AnalyticsErrorEventSource.BILLING_APPLY_COUPON_CODE_INPUT);
        } finally {
            isLoading.value = false;
        }
    });
}
</script>

<style scoped lang="scss">
    .add-coupon {
        font-family: 'font_regular', sans-serif;

        &__input-container {
            display: flex;
            flex-direction: column;
        }

        &__input-wrapper {
            margin: 10px 0;
        }

        &__valid-message {
            margin-bottom: 10px;
        }
    }
</style>

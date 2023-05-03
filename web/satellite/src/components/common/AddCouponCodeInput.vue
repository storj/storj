// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-coupon__input-container">
        <div v-if="!showConfirmMessage">
            <div class="add-coupon__input-wrapper">
                <img
                    class="add-coupon__input-icon"
                    :class="{'signup-view': isSignupView}"
                    src="@/../static/images/account/billing/coupon.png"
                    alt="Coupon"
                >
                <VInput
                    :label="inputLabel"
                    placeholder="Enter Coupon Code"
                    height="52px"
                    with-icon
                    @setData="setCouponCode"
                />
                <CheckIcon
                    v-if="isCodeValid"
                    class="add-coupon__check"
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
                v-if="!isSignupView"
                class="add-coupon__apply-button"
                label="Apply Coupon Code"
                width="100%"
                height="44px"
                :on-press="couponCheck"
            />
        </div>
        <div v-if="showConfirmMessage">
            <p class="add-coupon__confirm-message">
                By applying this coupon you will override your existing coupon.
                Are you sure you want to remove your current coupon and replace it with this new coupon?
            </p>
            <div class="add-coupon__button-wrapper">
                <VButton
                    label="Yes"
                    width="100%"
                    height="48px"
                    :on-press="applyCouponCode"
                />
                <VButton
                    label="Back"
                    width="calc(100% - 4px)"
                    height="44px"
                    :is-blue-white="true"
                    :on-press="toggleConfirmMessage"
                />
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue';

import { RouteConfig } from '@/router';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useRouter } from '@/utils/hooks';
import { useBillingStore } from '@/store/modules/billingStore';

import VInput from '@/components/common/VInput.vue';
import ValidationMessage from '@/components/common/ValidationMessage.vue';
import VButton from '@/components/common/VButton.vue';

import CheckIcon from '@/../static/images/common/validCheck.svg';

const billingStore = useBillingStore();
const nativeRouter = useRouter();
const router = reactive(nativeRouter);

const showValidationMessage = ref<boolean>(false);
const isCodeValid = ref<boolean>(false);
const showConfirmMessage = ref<boolean>(false);
const errorMessage = ref<string>('');
const couponCode = ref<string>('');

const analytics = new AnalyticsHttpApi();

/**
 * Signup view requires some unique styling and element text.
 */
const isSignupView = computed((): boolean => {
    return router.currentRoute.name === RouteConfig.Register.name;
});

/**
 * Returns label for input if in signup view
 * Depending on view.
 */
const inputLabel = computed((): string => {
    return isSignupView.value ? 'Add Coupon' : '';
});

/**
 * Sets code from input.
 */
function setCouponCode(value: string): void {
    couponCode.value = value;
}

/**
 * Toggles showing of coupon code replace confirmation message
 */
function toggleConfirmMessage(): void {
    showConfirmMessage.value = !showConfirmMessage.value;
}

/**
 * Check if coupon code is valid
 */
async function applyCouponCode(): Promise<void> {
    try {
        await billingStore.applyCouponCode(couponCode.value);
    } catch (error) {
        errorMessage.value = error.message;
        isCodeValid.value = false;
        showValidationMessage.value = true;
        analytics.errorEventTriggered(AnalyticsErrorEventSource.BILLING_APPLY_COUPON_CODE_INPUT);

        return;
    } finally {
        if (showConfirmMessage.value) toggleConfirmMessage();
    }

    isCodeValid.value = true;
    showValidationMessage.value = true;
}

/**
 * Check if user has a coupon code applied to their account before applying
 */
async function couponCheck(): Promise<void> {
    if (billingStore.state.coupon) {
        toggleConfirmMessage();
        return;
    }

    await applyCouponCode();
}
</script>

<style scoped lang="scss">
    .add-coupon {

        &__input-wrapper {
            position: relative;
        }

        &__valid-message {
            margin-top: 15px;
        }

        &__apply-button {
            margin-top: 15px;
        }

        &__check {
            position: absolute;
            right: 15px;
            bottom: 15px;
        }

        &__input-icon {
            position: absolute;
            top: 27px;
            z-index: 23;
            left: 25px;
            width: 25.01px;
            height: 16.67px;
        }

        &__input-icon.signup-view {
            top: 50px;
        }

        &__confirm-message {
            font-family: 'font_regular', sans-serif;
            font-size: 18px;
            margin-top: 35px;
        }

        &__button-wrapper {
            display: flex;
            margin-top: 30px;
            column-gap: 20px;

            @media screen and (max-width: 650px) {
                flex-direction: column;
                column-gap: unset;
                row-gap: 20px;
            }
        }
    }
</style>

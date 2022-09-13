// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-coupon__input-container">
        <div class="add-coupon__input-wrapper">
            <p class="add-coupon__input-wrapper__label">
                Coupon Code:
            </p>
            <VInput
                :label="inputLabel"
                placeholder="Enter Coupon Code"
                height="52px"
                :with-icon="false"
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
            width="150px"
            height="44px"
            font-size="14px"
            :on-press="couponCheck"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { PaymentsHttpApi } from '@/api/payments';
import { RouteConfig } from '@/router';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';

import VInput from '@/components/common/VInput.vue';
import ValidationMessage from '@/components/common/ValidationMessage.vue';
import VButton from '@/components/common/VButton.vue';

import CheckIcon from '@/../static/images/common/validCheck.svg';

// @vue/component
@Component({
    components: {
        VButton,
        VInput,
        CheckIcon,
        ValidationMessage,
    },
})
export default class AddCouponCodeInput extends Vue {
    private showValidationMessage = false;
    private errorMessage = '';
    private isCodeValid = false;

    private couponCode = '';

    private readonly payments: PaymentsHttpApi = new PaymentsHttpApi();

    /**
     * Signup view requires some unque styling and element text.
     */
    public get isSignupView(): boolean {
        return this.$route.name === RouteConfig.Register.name;
    }

    /**
     * Returns label for input if in signup view
     * Depending on view.
     */
    public get inputLabel(): string | void {
        return this.isSignupView ? 'Add Coupon' : '';
    }

    public setCouponCode(value: string): void {
        this.couponCode = value;
    }

    /**
     * Check if coupon code is valid
     */
    public async applyCouponCode(): Promise<void> {
        try {
            await this.$store.dispatch(PAYMENTS_ACTIONS.APPLY_COUPON_CODE, this.couponCode);
        }
        catch (error) {
            this.errorMessage = error.message;
            this.isCodeValid = false;
            this.showValidationMessage = true;
            return;
        }
        this.isCodeValid = true;
        this.showValidationMessage = true;
    }

    /**
     * Check if user has a coupon code applied to their account before applying
     */
    public async couponCheck(): Promise<void> {
        try {
            this.applyCouponCode();
        } catch (error) {
            this.errorMessage = error.message;
            this.isCodeValid = false;
            this.showValidationMessage = true;
            return;
        }
    }
}
</script>

<style scoped lang="scss">
    .add-coupon {

        &__input-container {
            display: flex;
            flex-direction: column;
        }

        &__input-wrapper {
            position: relative;
            margin: 30px 0 10px;

            &__label {
                font-family: sans-serif;
                font-weight: 700;
                font-size: 14px;
                color: #56606d;
                margin-top: -15px;
            }
        }

        &__valid-message {
            position: relative;
            margin-bottom: 10px;
        }

        &__apply-button {
            left: 0;
            right: 0;
            margin: 0 auto;
            bottom: 20px;
            background: #2683ff;

            &:hover {
                background: #0059d0;
            }
        }

        &__check {
            position: absolute;
            right: 15px;
            bottom: 15px;
        }

        &__confirm-message {
            font-family: 'font_regular', sans-serif;
            font-size: 18px;
            margin-top: 35px;
        }

        &__button-wrapper {
            display: flex;
            justify-content: space-evenly;
            margin-top: 30px;
        }
    }

</style>

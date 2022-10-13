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
                    :with-icon="true"
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
                width="85%"
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
                    class="add-coupon__confirm-button"
                    label="Yes"
                    width="250px"
                    height="44px"
                    :on-press="applyCouponCode"
                />
                <VButton
                    class="add-coupon__back-button"
                    label="Back"
                    width="250px"
                    height="44px"
                    is-blue-white="true"
                    :on-press="toggleConfirmMessage"
                />
            </div>
        </div>
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

    private showConfirmMessage = false;

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
    * Toggles showing of coupon code replace confirmation message
    */
    public toggleConfirmMessage(): void {
        this.showConfirmMessage = !this.showConfirmMessage;
    }

    /**
     * Check if coupon code is valid
     */
    public async applyCouponCode(): Promise<void> {
        try {
            await this.$store.dispatch(PAYMENTS_ACTIONS.APPLY_COUPON_CODE, this.couponCode);
        }
        catch (error) {
            if (this.showConfirmMessage) {
                this.toggleConfirmMessage();
            }
            this.errorMessage = error.message;
            this.isCodeValid = false;
            this.showValidationMessage = true;

            return;
        }
        if (this.showConfirmMessage) {
            this.toggleConfirmMessage();
        }
        this.isCodeValid = true;
        this.showValidationMessage = true;
    }

    /**
     * Check if user has a coupon code applied to their account before applying
     */
    public async couponCheck(): Promise<void> {
        try {
            if (this.$store.state.paymentsModule.coupon) {
                this.toggleConfirmMessage();
            } else {
                this.applyCouponCode();
            }
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

        &__input-wrapper {
            position: relative;
        }

        &__valid-message {
            position: relative;
            top: 15px;
        }

        &__apply-button {
            position: absolute;
            left: 0;
            right: 0;
            margin: 0 auto;
            bottom: 40px;
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

        &__input-icon {
            position: absolute;
            top: 27px;
            z-index: 23;
            left: 25px;
            width: 25.01px;
            height: 16.67px;
        }

        &__input-icon.signup-view {
            top: 63px;
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

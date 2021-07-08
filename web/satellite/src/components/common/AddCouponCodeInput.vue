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
                />
                <HeaderlessInput
                    :label="inputLabel"
                    placeholder="Enter Coupon Code"
                    class="add-coupon__input"
                    height="52px"
                    :withIcon="true"
                    @setData="setCouponCode"
                />
                <CheckIcon
                    class="add-coupon__check"
                    v-if="isCodeValid"
                />
            </div>
            <ValidationMessage
                class="add-coupon__valid-message"
                successMessage="Successfully applied coupon code."
                :errorMessage="errorMessage"
                :isValid="isCodeValid"
                :showMessage="showValidationMessage"
            />
            <VButton
                class="add-coupon__apply-button"
                label="Apply Coupon Code"
                width="85%"
                height="44px"
                v-if="!isSignupView"
                :on-press="couponCheck"
            />
        </div>
        <div v-if="showConfirmMessage">
            <p class="add-coupon__confirm-message">By applying this coupon you will override your existing coupon.
            Are you sure you want to remove your current coupon and replace it with this new coupon?</p>
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
                    isBlueWhite="true"
                    :on-press="toggleConfirmMessage"
                />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
import ValidationMessage from '@/components/common/ValidationMessage.vue';
import VButton from '@/components/common/VButton.vue';

import CloseIcon from '@/../static/images/common/closeCross.svg';
import CheckIcon from '@/../static/images/common/validCheck.svg';

import { PaymentsHttpApi } from '@/api/payments';
import { RouteConfig } from '@/router';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';

const {
    APPLY_COUPON_CODE,
} = PAYMENTS_ACTIONS;

@Component({
    components: {
        VButton,
        HeaderlessInput,
        CloseIcon,
        CheckIcon,
        ValidationMessage,
    },
})
export default class AddCouponCodeInput extends Vue {

    @Prop({default: false})
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
            await this.payments.applyCouponCode(this.couponCode);
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
            const userCouponCodes = await this.payments.hasCouponApplied();
            if (userCouponCodes) {
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

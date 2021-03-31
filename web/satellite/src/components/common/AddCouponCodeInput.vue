// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-coupon__warpperr">
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
            <VButton
                label="Validate"
                class="add-coupon__claim-button"
                width="120px"
                height="32px"
                v-if="!isCodeValid"
                :on-press="onValidationCheckClick"
            />
        </div>
        <ValidationMessage
            class="add-coupon__valid-message"
            successMessage="Get 50GB free and start using Tardigrade today."
            errorMessage="Invalid code. Please Try again"
            :isValid="isCodeValid"
            :showMessage="showValidationMessage"
        />
        <VButton
            class="add-coupon__apply-button"
            label="Apply Coupon Code"
            width="85%"
            height="44px"
            v-if="!isSignupView"
        />
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
import ValidationMessage from '@/components/common/ValidationMessage.vue';
import VButton from '@/components/common/VButton.vue';

import CloseIcon from '@/../static/images/common/closeCross.svg';
import CheckIcon from '@/../static/images/common/validCheck.svg';

import { RouteConfig } from '@/router';

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
    protected readonly isCodeValid: boolean;
    @Prop({default: false})
    protected readonly showValidationMessage: boolean;

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

    /**
    * Check if coupon code is valid
    */
    public onValidationCheckClick(): boolean {
        return true;
    }

}
</script>

<style scoped lang="scss">

    .add-coupon {

        &__wrapper {
            background: #1b2533c7 75%;
            position: fixed;
            width: 100%;
            height: 100%;
            top: 0;
            left: 0;
        }

        &__input-wrapper {
            position: relative;
        }

        &__claim-button {
            position: absolute;
            right: 12px;
            bottom: 8px;
            font-size: 14px;
            padding: 2px 0;
            z-index: 23;
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
            background: #93a1af;

            &:hover {
                background: darken(#93a1af, 10%);
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
    }

</style>

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

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import VInput from '@/components/common/VInput.vue';
import ValidationMessage from '@/components/common/ValidationMessage.vue';
import VButton from '@/components/common/VButton.vue';

// @vue/component
@Component({
    components: {
        VButton,
        VInput,
        ValidationMessage,
    },
})
export default class NewBillingAddCouponCodeInput extends Vue {
    private showValidationMessage = false;
    private isCodeValid = false;
    private errorMessage = '';
    private couponCode = '';
    private isLoading = false;

    private readonly analytics = new AnalyticsHttpApi();

    public setCouponCode(value: string): void {
        this.couponCode = value;
    }

    /**
     * Check if coupon code is valid
     */
    public async applyCouponCode(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        try {
            await this.$store.dispatch(PAYMENTS_ACTIONS.APPLY_COUPON_CODE, this.couponCode);
            await this.$notify.success('Coupon Added!');
            this.$emit('close');
        } catch (error) {
            this.errorMessage = error.message;
            this.isCodeValid = false;
            this.showValidationMessage = true;
            this.analytics.errorEventTriggered(AnalyticsErrorEventSource.BILLING_APPLY_COUPON_CODE_INPUT);
        } finally {
            this.isLoading = false;
        }
    }
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

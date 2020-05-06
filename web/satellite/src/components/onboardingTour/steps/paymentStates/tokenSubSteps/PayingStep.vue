// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="paying-step">
        <div class="paying-step__title-area">
            <img src="@/../static/images/onboardingTour/coins.png" alt="coins image">
            <h2 class="paying-step__title-area__title">
                Select Your Deposit Amount
            </h2>
            <img
                v-if="isLoading"
                class="loading-image"
                src="@/../static/images/account/billing/loading.gif"
                alt="loading gif"
            >
        </div>
        <TokenDepositSelection
            class="paying-step__form"
            @onChangeTokenValue="onChangeTokenValue"
        />
        <VButton
            width="100%"
            height="48px"
            label="Continue to Coin Payments"
            :on-press="onConfirmAddSTORJ"
            :is-blue-white="true"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import TokenDepositSelection from '@/components/account/billing/paymentMethods/TokenDepositSelection.vue';
import VButton from '@/components/common/VButton.vue';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';

const {
    MAKE_TOKEN_DEPOSIT,
    GET_BILLING_HISTORY,
} = PAYMENTS_ACTIONS;

@Component({
    components: {
        VButton,
        TokenDepositSelection,
    },
})

export default class PayingStep extends Vue {
    private readonly DEFAULT_TOKEN_DEPOSIT_VALUE = 50; // in dollars.
    private readonly MAX_TOKEN_AMOUNT = 1000000; // in dollars.
    private tokenDepositValue: number = this.DEFAULT_TOKEN_DEPOSIT_VALUE;

    public isLoading: boolean = false;

    /**
     * Event for changing token deposit value.
     */
    public onChangeTokenValue(value: number): void {
        this.tokenDepositValue = value;
    }

    /**
     * onConfirmAddSTORJ checks if amount is valid and if so process token.
     * payment and return state to default
     */
    public async onConfirmAddSTORJ(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;
        this.$emit('toggleIsLoading');

        if (this.tokenDepositValue < this.DEFAULT_TOKEN_DEPOSIT_VALUE || this.tokenDepositValue >= this.MAX_TOKEN_AMOUNT) {
            await this.$notify.error('First deposit amount must be more than $50 and less than $1000000');
            this.setDefaultState();

            return;
        }

        try {
            const tokenResponse = await this.$store.dispatch(MAKE_TOKEN_DEPOSIT, this.tokenDepositValue * 100);
            await this.$notify.success(`Successfully created new deposit transaction! \nAddress:${tokenResponse.address} \nAmount:${tokenResponse.amount}`);
            const depositWindow = window.open(tokenResponse.link, '_blank');
            if (depositWindow) {
                depositWindow.focus();
            }

        } catch (error) {
            await this.$notify.error(error.message);
            this.setDefaultState();
        }

        this.$segment.track(SegmentEvent.PAYMENT_METHOD_ADDED, {
            project_id: this.$store.getters.selectedProject.id,
        });

        this.tokenDepositValue = this.DEFAULT_TOKEN_DEPOSIT_VALUE;
        try {
            await this.$store.dispatch(GET_BILLING_HISTORY);
        } catch (error) {
            await this.$notify.error(error.message);
        }

        this.setDefaultState();
        this.$emit('setVerifyingState');
    }

    /**
     * Sets area to default state.
     */
    private setDefaultState(): void {
        this.isLoading = false;
        this.$emit('toggleIsLoading');
    }
}
</script>

<style scoped lang="scss">
    p,
    h2 {
        margin: 0;
    }

    .paying-step {
        display: flex;
        flex-direction: column;
        width: 60%;

        &__title-area {
            display: flex;
            align-items: center;
            justify-content: flex-start;
            margin-bottom: 25px;

            &__title {
                font-size: 20px;
                line-height: 26px;
                color: #384b65;
                margin-left: 15px;
            }
        }

        &__form {
            width: 100%;
            margin-bottom: 20px;

            /deep/ .selected-container,
            /deep/ .options-container,
            /deep/ .payment-selection-blur {
                width: 100%;
            }

            /deep/ .custom-input {
                width: calc(100% - 61px);
            }
        }
    }

    .loading-image {
        width: 18px;
        height: 18px;
        margin-left: 10px;
    }
</style>
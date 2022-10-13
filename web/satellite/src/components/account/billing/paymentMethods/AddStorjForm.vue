// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-storj-area">
        <p class="add-storj-area__support-info">To deposit STORJ token and request higher limits, please contact <a target="_blank" rel="noopener noreferrer" href="https://supportdcs.storj.io/hc/en-us/requests/new?ticket_form_id=360000683212">Support</a></p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PaymentAmountOption } from '@/types/payments';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

const {
    MAKE_TOKEN_DEPOSIT,
    GET_PAYMENTS_HISTORY,
} = PAYMENTS_ACTIONS;

// @vue/component
@Component
export default class AddStorjForm extends Vue {
    private readonly DEFAULT_TOKEN_DEPOSIT_VALUE = 10; // in dollars.
    private readonly MAX_TOKEN_AMOUNT = 1000000; // in dollars.
    private tokenDepositValue: number = this.DEFAULT_TOKEN_DEPOSIT_VALUE;

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    @Prop({ default: false })
    public readonly isLoading: boolean;

    /**
     * Set of default payment options.
     */
    public paymentOptions: PaymentAmountOption[] = [
        new PaymentAmountOption(10, `USD $10`),
        new PaymentAmountOption(20, `USD $20`),
        new PaymentAmountOption(50, `USD $50`),
        new PaymentAmountOption(100, `USD $100`),
        new PaymentAmountOption(1000, `USD $1000`),
    ];

    /**
     * onConfirmAddSTORJ checks if amount is valid.
     * If so processes token payment and returns state to default.
     */
    public async onConfirmAddSTORJ(): Promise<void> {
        this.$emit('toggleIsLoading');

        if (!this.isDepositValueValid) return;

        try {
            this.analytics.eventTriggered(AnalyticsEvent.STORJ_TOKEN_ADDED_FROM_BILLING);
            const tokenResponse = await this.$store.dispatch(MAKE_TOKEN_DEPOSIT, this.tokenDepositValue * 100);
            await this.$notify.success(`Successfully created new deposit transaction! \nAddress:${tokenResponse.address} \nAmount:${tokenResponse.amount}`);
            const depositWindow = window.open(tokenResponse.link, '_blank');
            if (depositWindow) {
                depositWindow.focus();
            }
        } catch (error) {
            await this.$notify.error(error.message);
            this.$emit('toggleIsLoading');
        }

        this.tokenDepositValue = this.DEFAULT_TOKEN_DEPOSIT_VALUE;
        try {
            await this.$store.dispatch(GET_PAYMENTS_HISTORY);
        } catch (error) {
            await this.$notify.error(error.message);
            this.$emit('toggleIsLoading');
        }

        this.$emit('toggleIsLoading');
        this.$emit('cancel');
    }

    /**
     * Event for changing token deposit value.
     */
    public onChangeTokenValue(value: number): void {
        this.tokenDepositValue = value;
    }

    /**
     * Indicates if user has own project.
     */
    private get userHasOwnProject(): boolean {
        return this.$store.getters.projectsCount > 0;
    }

    /**
     * Indicates if deposit value is valid.
     */
    private get isDepositValueValid(): boolean {
        switch (true) {
        case (this.tokenDepositValue < this.DEFAULT_TOKEN_DEPOSIT_VALUE || this.tokenDepositValue >= this.MAX_TOKEN_AMOUNT) && !this.userHasOwnProject:
            this.$notify.error('First deposit amount must be more than $10 and less than $1000000');
            this.setDefault();

            return false;
        case this.tokenDepositValue >= this.MAX_TOKEN_AMOUNT || this.tokenDepositValue === 0:
            this.$notify.error('Deposit amount must be more than $0 and less than $1000000');
            this.setDefault();

            return false;
        default:
            return true;
        }
    }

    /**
     * Sets adding payment method state to default.
     */
    private setDefault(): void {
        this.tokenDepositValue = this.DEFAULT_TOKEN_DEPOSIT_VALUE;
        this.$emit('toggleIsLoading');
        this.$emit('cancel');
    }
}
</script>

<style scoped lang="scss">
    p {
        margin: 0;
    }

    .add-storj-area {
        margin: 20px 0;
        font-family: 'font_regular', sans-serif;
        display: flex;
        max-height: 52px;
        justify-content: space-between;
        align-items: center;

        &__selection-container {
            display: flex;
            align-items: center;

            &__label {
                margin-right: 30px;
                max-width: 215px;
            }

            &__form {
                width: 60%;
            }
        }

        &__submit-area {
            display: flex;
            align-items: center;
            min-width: 135px;
        }

        &__support-info {
            font-weight: 600;
            font-size: 14px;
            line-height: 20px;
            color: #000;

            a {
                color: #0149ff;
            }
        }
    }

    .loading-image {
        width: 18px;
        height: 18px;
        margin-right: 5px;
    }
</style>

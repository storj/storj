// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-storj-area">
        <div class="add-storj-area__selection-container">
            <p class="add-storj-area__selection-container__label">Deposit STORJ Tokens via Coin Payments</p>
            <TokenDepositSelection
                class="add-storj-area__selection-container__form"
                @onChangeTokenValue="onChangeTokenValue"
            />
        </div>
        <div class="add-storj-area__submit-area">
            <img
                v-if="isLoading"
                class="loading-image"
                src="@/../static/images/account/billing/loading.gif"
                alt="loading gif"
            >
            <VButton
                class="confirm-add-storj-button"
                label="Continue to Coin Payments"
                width="251px"
                height="48px"
                :on-press="onConfirmAddSTORJ"
                :is-disabled="isLoading"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import TokenDepositSelection from '@/components/account/billing/paymentMethods/TokenDepositSelection.vue';
import VButton from '@/components/common/VButton.vue';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';
import { ProjectOwning } from '@/utils/projectOwning';

const {
    MAKE_TOKEN_DEPOSIT,
    GET_BILLING_HISTORY,
} = PAYMENTS_ACTIONS;

@Component({
    components: {
        TokenDepositSelection,
        VButton,
    },
})
export default class AddStorjForm extends Vue {
    private readonly DEFAULT_TOKEN_DEPOSIT_VALUE = 50; // in dollars.
    private readonly MAX_TOKEN_AMOUNT = 1000000; // in dollars.
    private tokenDepositValue: number = this.DEFAULT_TOKEN_DEPOSIT_VALUE;

    @Prop({ default: false })
    public readonly isLoading: boolean;

    /**
     * onConfirmAddSTORJ checks if amount is valid and if so process token.
     * payment and return state to default
     */
    public async onConfirmAddSTORJ(): Promise<void> {
        this.$emit('toggleIsLoading');

        if (!this.isDepositValueValid) return;

        try {
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

        this.$segment.track(SegmentEvent.PAYMENT_METHOD_ADDED, {
            project_id: this.$store.getters.selectedProject.id,
        });

        this.tokenDepositValue = this.DEFAULT_TOKEN_DEPOSIT_VALUE;
        try {
            await this.$store.dispatch(GET_BILLING_HISTORY);
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
        return new ProjectOwning(this.$store).userHasOwnProject();
    }

    /**
     * Indicates if deposit value is valid.
     */
    private get isDepositValueValid(): boolean {
        switch (true) {
            case (this.tokenDepositValue < this.DEFAULT_TOKEN_DEPOSIT_VALUE || this.tokenDepositValue >= this.MAX_TOKEN_AMOUNT) && !this.userHasOwnProject:
                this.$notify.error('First deposit amount must be more than $50 and less than $1000000');
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
        margin-top: 44px;
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
    }

    .loading-image {
        width: 18px;
        height: 18px;
        margin-right: 5px;
    }
</style>

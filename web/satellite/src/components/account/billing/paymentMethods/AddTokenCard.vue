// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="token">
        <div class="token__large-icon-container">
            <div class="token__large-icon">
                <StorjLarge />
            </div>
        </div>
        <v-loader v-if="!pageLoaded" class="token-loader" />
        <div v-else class="token__add-funds">
            <h3 class="token__add-funds__title">
                STORJ Token
            </h3>
            <div class="token__add-funds__button-container">
                <p class="token__add-funds__support-info">To deposit STORJ token and request higher limits, please contact <a target="_blank" rel="noopener noreferrer" href="https://supportdcs.storj.io/hc/en-us/requests/new?ticket_form_id=360000683212">Support</a></p>
                <VButton
                    v-if="totalCount > 0"
                    class="token__add-funds__button"
                    label="Back"
                    is-transparent="true"
                    width="50px"
                    height="30px"
                    font-size="11px"
                    :on-press="toggleShowAddFunds"
                />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import StorjLarge from '@/../static/images/billing/storj-icon-large.svg';
import VButton from '@/components/common/VButton.vue';
import VLoader from '@/components/common/VLoader.vue';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PaymentAmountOption } from '@/types/payments';

const {
    MAKE_TOKEN_DEPOSIT,
    GET_PAYMENTS_HISTORY,
} = PAYMENTS_ACTIONS;

// @vue/component
@Component({
    components: {
        StorjLarge,
        VButton,
        VLoader,
    },
})
export default class AddTokenCard extends Vue {
    @Prop({default: 0})
    private readonly totalCount: number;
    private readonly DEFAULT_TOKEN_DEPOSIT_VALUE = 10; // in dollars.
    private readonly MAX_TOKEN_AMOUNT = 1000000; // in dollars.
    private tokenDepositValue: number = this.DEFAULT_TOKEN_DEPOSIT_VALUE;

    private pageLoaded = true;

    public toggleShowAddFunds(): void {
        this.$emit("toggleShowAddFunds");
    }

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

        if (!this.isDepositValueValid) return;

        try {
            this.pageLoaded = false;
            const tokenResponse = await this.$store.dispatch(MAKE_TOKEN_DEPOSIT, this.tokenDepositValue * 100);
            await this.$notify.success(`Successfully created new deposit transaction! \nAddress:${tokenResponse.address} \nAmount:${tokenResponse.amount}`);
            const depositWindow = window.open(tokenResponse.link, '_blank');
            if (depositWindow) {
                depositWindow.focus();
                this.pageLoaded = true;
            }
            this.$emit("fetchHistory");
        } catch (error) {
            await this.$notify.error(error.message);
            this.$emit('toggleIsLoading');
            this.pageLoaded = true;
        }

        this.tokenDepositValue = this.DEFAULT_TOKEN_DEPOSIT_VALUE;
        try {
            await this.$store.dispatch(GET_PAYMENTS_HISTORY);
        } catch (error) {
            await this.$notify.error(error.message);
            this.$emit('toggleIsLoading');
        }
    }

    /**
     * Event for changing token deposit value.
     */
    public onChangeTokenValue(value: number): void {
        this.tokenDepositValue = value;
    }

    /**
     * Indicates if deposit value is valid.
     */
    private get isDepositValueValid(): boolean {
        switch (true) {
        case (this.tokenDepositValue < this.DEFAULT_TOKEN_DEPOSIT_VALUE || this.tokenDepositValue >= this.MAX_TOKEN_AMOUNT) && !this.userHasOwnProject:
            this.$notify.error(`First deposit amount must be more than $10 and less than $${this.MAX_TOKEN_AMOUNT}`);
            this.setDefault();

            return false;
        case this.tokenDepositValue >= this.MAX_TOKEN_AMOUNT || this.tokenDepositValue === 0:
            this.$notify.error(`Deposit amount must be more than $0 and less than $${this.MAX_TOKEN_AMOUNT}`);
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
    }

    /**
     * Indicates if user has own project.
     */
    private get userHasOwnProject(): boolean {
        return this.$store.getters.projectsCount > 0;
    }
}
</script>

<style scoped lang="scss">
    .token-loader {
        width: 100% !important;
        padding: 0 !important;
        margin: 40px 0;
    }

    .token {
        border-radius: 10px;
        width: 227px;
        height: 126px;
        margin: 0 10px 10px 0;
        padding: 20px;
        box-shadow: 0 0 20px rgb(0 0 0 / 4%);
        background: #fff;
        position: relative;

        &__large-icon-container {
            position: absolute;
            top: 0;
            right: 0;
            height: 120px;
            width: 120px;
            z-index: 1;
            border-radius: 10px;
            overflow: hidden;
        }

        &__large-icon {
            position: absolute;
            top: -25px;
            right: -24px;
        }

        &__add-funds {
            display: flex;
            flex-direction: column;
            z-index: 5;
            position: relative;
            height: 100%;
            width: 100%;

            &__title {
                font-family: sans-serif;
            }

            &__label {
                font-family: sans-serif;
                color: #56606d;
                font-size: 11px;
                margin-top: 5px;
            }

            &__dropdown {
                margin-top: 10px;
            }

            &__button-container {
                margin-top: 10px;
                display: flex;
                justify-content: space-between;
            }

            &__support-info {
                font-family: sans-serif;
                font-weight: 600;
                font-size: 14px;
                line-height: 20px;
                color: #000;
                position: relative;
                top: 14px;

                a {
                    color: #0149ff;
                }
            }
        }
    }
</style>

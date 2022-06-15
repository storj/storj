// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payments-area">
        <div class="payments-area__top-container">
            <h1 class="payments-area__title">Payment Methods</h1>
            <VLoader v-if="ispaymentsFetching" />
            <div class="payments-area__container">
                <div
                    class="payments-area__container__token"
                >
                    <div class="payments-area__container__token__small-icon">   <StorjSmall />
                    </div>
                    <div class="payments-area__container__token__large-icon">   <StorjLarge />
                    </div>
                    <div class="payments-area__container__token__confirmation-container">
                        <p class="payments-area__container__token__confirmation-container__label">STORJ Token Deposit</p>
                        <span :class="`payments-area__container__token__confirmation-container__circle-icon ${depositStatus}`">
                            &#9679;
                        </span>
                        <span class="payments-area__container__token__confirmation-container__text">
                            <span>
                                {{ depositStatus }}
                            </span>
                        </span>
                    </div>

                    
                    <div class="payments-area__container__token__balance-container">
                        <p class="payments-area__container__token__balance-container__label">Total Balance</p>
                        <span class="payments-area__container__token__balance-amount">USD $ {{ balanceAmount }}</span>
                    </div>
                    <div class="payments-area__container__token__button-container">
                        <v-button
                            label='See transactions'
                            width="auto"
                            height="30px"
                            is-transparent="true"
                            font-size="13px"
                            class=""
                        />
                        <v-button
                            label='Add funds'
                            font-size="13px"
                            width="auto"
                            height="30px"
                            class=""
                        />
                    </div>
                </div>
                <div class="payments-area__container__cards" />
                <div class="payments-area__container__new-payments">
                    <div v-if="!isAddingPayment" class="payments-area__container__new-payments__text-area">
                        <span class="payments-area__container__new-payments__text-area__plus-icon">+&nbsp;</span>
                        <span 
                            class="payments-area__container__new-payments__text-area__text"
                            @click="addPaymentMethodHandler"
                        >Add New Payment Method</span>
                    </div>
                    <div v-if="isAddingPayment">
                        <div class="payments-area__create-header">Credit Card</div>
                        <div class="payments-area__create-subheader">Add Card Info</div>
                        <StripeCardInput
                            ref="stripeCardInput"
                            class="add-card-area__stripe stripe_input"
                            :on-stripe-response-callback="addCard"
                        />
                        <div
                            v-if="!isAddCardClicked"
                            class="add-card-button"
                            @click="addCard"
                        >
                            <img
                                v-if="isLoading"
                                class="payment-loading-image"
                                src="@/../static/images/account/billing/loading.gif"
                                alt="loading gif"
                            >
                            <SuccessImage
                                v-if="isLoaded"
                                class="payment-loaded-image"
                            />
                            <span class="add_card_button_text">Add Credit Card</span>
                        </div>
                    </div>
                </div>
            </div>
            <!-- Edit Credit Card Modal -->
            <div v-if="isEditPaymentMethodsModalOpen" class="add_payment_method">
                <div class="add_payment_method__container">
                    <CreditCard  class="card-icon" />
                    <div class="add_payment_method__header">Add Credit Card</div>
                    <div class="add_payment_method__header-subtext">This is not your default payment card.</div>
                    <div class="add_payment_method__container__close-cross-container" @click="onCloseClick">
                        <CloseCrossIcon />
                    </div>
                    <form>
                        <label>Card Number</label>
                    </form>
                </div>
            </div>
            <Addpayments2 
                v-if="showCreateCode"
                @toggleMethod="toggleCreateModal"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VLoader from '@/components/common/VLoader.vue';
import VButton from '@/components/common/VButton.vue';
import CloseCrossIcon from '@/../static/images/common/closeCross.svg';
import StripeCardInput from '@/components/account/billing/paymentMethods/StripeCardInput.vue';
import SuccessImage from '@/../static/images/account/billing/success.svg';

import StorjSmall from '@/../static/images/billing/storj-icon-small.svg';
import StorjLarge from '@/../static/images/billing/storj-icon-large.svg';
import CreditCard from '@/../static/images/billing/credit-card.svg';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { USER_ACTIONS } from '@/store/modules/users';

import { RouteConfig } from '@/router';


interface StripeForm {
    onSubmit(): Promise<void>;
}
const {
    ADD_CREDIT_CARD,
    GET_CREDIT_CARDS,
} = PAYMENTS_ACTIONS;

// @vue/component
@Component({
    components: {
        VLoader,
        StorjSmall,
        StorjLarge,
        VButton,
        CloseCrossIcon,
        CreditCard,
        StripeCardInput,
        SuccessImage
    },
})
export default class paymentsArea extends Vue {
    public isAddingPayment = false;
    public isEditPaymentMethodsModalOpen = false;
    public depositStatus = 'Confirmed';
    public balanceAmount = 0.00; 
    public testData = [{},{},{}];
    public isAddCardClicked = false;
    public $refs!: {
        stripeCardInput: StripeCardInput & StripeForm;
    };
    /**
     * Lifecycle hook after initial render.
     * Fetches payments.
     */
    public async mounted() {
    }

    

    public async addCard(token: string): Promise<void> {
        this.$emit('toggleIsLoading');
        console.log('working');
        console.log(token, 'token');
        try {
        console.log('catch block firing');
            await this.$store.dispatch(ADD_CREDIT_CARD, token);

            // We fetch User one more time to update their Paid Tier status.
            await this.$store.dispatch(USER_ACTIONS.GET);
        } catch (error) {
            console.log('catching');
            await this.$notify.error(error.message);

            this.$emit('toggleIsLoading');

            return;
        }

        await this.$notify.success('Card successfully added');
        try {
            await this.$store.dispatch(GET_CREDIT_CARDS);
        } catch (error) {
            await this.$notify.error(error.message);
            this.$emit('toggleIsLoading');
        }

        this.$emit('toggleIsLoading');
        this.$emit('toggleIsLoaded');

        setTimeout(() => {
            this.$emit('cancel');
            this.$emit('toggleIsLoaded');

            setTimeout(() => {
                if (!this.userHasOwnProject) {
                    this.$router.push(RouteConfig.CreateProject.path);
                }
            }, 500);
        }, 2000);
    }
    private get userHasOwnProject(): boolean {
        return this.$store.getters.projectsCount > 0;
    }

    public async onConfirmAddStripe(): Promise<void> {
        await this.$refs.stripeCardInput.onSubmit();
    }

    public addPaymentMethodHandler() {
        this.isAddingPayment = true;
    }

    public editPaymentMethodHandler() {
        this.isEditPaymentMethodsModalOpen = true;
    }

    public onCloseClick() {
        this.isEditPaymentMethodsModalOpen = false;
    }
}
</script>

<style scoped lang="scss">

    .payments-area__container__new-payments {
        display: grid !important;
        grid-template-columns:  6fr;
        grid-template-rows: 1fr 1fr 1fr 1fr;
    }

    .card-icon {
        position: absolute;
        left: 45.78%;
        top: 14.00%;
        bottom: 67.38%;
    }

     .add_payment_method {
        // width: 546px;
        position: fixed;
        top: 0;
        bottom: 0;
        left: 0;
        right: 0;
        z-index: 100;
        background: rgb(27 37 51 / 75%);
        display: flex;
        align-items: center;
        justify-content: center;

        &__header {
            position: absolute;
            left: 36%;
            top: 24%;
            font-family: sans-serif;
            font-style: normal;
            font-weight: 800;
            font-size: 24px;
            line-height: 31px;
            /* or 129% */

            text-align: center;
            letter-spacing: -0.02em;

            color: #1B2533;
        }
        &__header-subtext {
            position: absolute;
            left: 30%;
            top: 29%;
            font-family: sans-serif;
            font-style: normal;
            font-weight: 400;
            font-size: 14px;
            line-height: 20px;
            text-align: center;
            color: #56606D;
        }

        &__container {
            width: 546px;
            height: 550px;

            background: #f5f6fa;
            border-radius: 6px;
            display: flex;
            align-items: flex-start;
            position: relative;

            &__close-cross-container {
                display: flex;
                justify-content: center;
                align-items: center;
                position: absolute;
                right: 30px;
                top: 30px;
                height: 24px;
                width: 24px;
                cursor: pointer;

                &:hover .close-cross-svg-path {
                    fill: #2683ff;
                }
            }
        }
    }

    .add-card-button {
        grid-row: 4;
        grid-column: 1;
        width: 115px;
        height: 30px;
        margin-top: 16px;
        
        cursor: pointer;
        border-radius: 6px;
        background-color: #0149FF;
        font-family: 'font_medium', sans-serif;
        font-size: 16px;
        line-height: 23px;
        color: #fff;
        user-select: none;
        transition: top 0.5s ease-in-out;

        &:hover {
            background-color: #0059d0;
        }
    }
    
    .active-discount {
        background: #dffff7;
        color: #00ac26;
    }

    .add_card_button_text {
        margin-left: 9px;
        font-family: font-medium, sans-serif;
        font-style: normal;
        font-weight: 700;
        font-size: 13px;
        line-height: 29px;
        /* identical to box height, or 154% */

        display: flex;
        align-items: center;
        letter-spacing: -0.02em;
        
    }

    .inactive-discount {
        background: #ffe1df;
        color: #ac1a00;
    }

    .active-status {
        background: #00ac26;
    }

    .inactive-status {
        background: #ac1a00;
    }

    .stripe_input {
        grid-row: 3;
        grid-column: 1;
        width: 260px;
        margin-top: 10px;
    }

    .payments-area {

        &__create-header {
            grid-row: 1;
            grid-column: 1;
            font-family: sans-serif;
            font-style: normal;
            font-weight: 700;
            font-size: 18px;
            line-height: 27px;
        }

        &__create-subheader {
            grid-row: 2;
            grid-column: 1;
            
            font-family: sans-serif;
            font-style: normal;
            font-weight: 400;
            font-size: 14px;
            line-height: 20px;
            color: #56606D;
        }


        &__title {
            font-family: sans-serif;
            font-size: 24px;
            margin: 20px 0;
        }

        &__container {
            display: flex;
            flex-wrap: wrap;

            &__token {
                border-radius: 10px;
                max-width: 400px;
                width: 18vw;
                min-width: 227px;
                max-height: 222px;
                height: 10vw;
                min-height: 126px;
                display: grid;
                grid-template-columns: 2fr 1fr 1fr;
                grid-template-rows: 1fr 1fr 1fr;
                margin: 0 10px 10px 0;
                padding: 20px;
                box-shadow: 0 0 20px rgb(0 0 0 / 4%);
                background: #fff;
                overflow: hidden;
                &__small-icon{
                    grid-column: 1;
                    grid-row: 1;
                    height: 30px;
                    width: 40px;
                    background-color: #E6EDF7;
                    border-radius: 5px;
                    display: flex;
                    justify-content: center;
                    align-items: center;
                }
                &__large-icon{
                    grid-column: 1/3;
                    grid-row: 1/3;
                    margin: 0 0 auto 0;
                    position: relative;
                    top: -50px;
                    right: -130px;
                    z-index: 2;
                }

                &__confirmation-container {
                    grid-column: 1;
                    grid-row: 2;
                    z-index: 3;
                }
                &__balance-container {
                    grid-column: 2;
                    grid-row: 2;
                    z-index: 3;
                }
                &__button-container{
                    grid-column: 1/3;
                    grid-row: 4;
                    z-index: 3;
                }
            }

            &__new-payments {
                border: 2px dashed #929fb1;
                border-radius: 10px;
                max-width: 400px;
                width: 18vw;
                min-width: 227px;
                max-height: 222px;
                height: 10vw;
                min-height: 126px;
                padding: 18px;
                display: flex;
                align-items: center;
                justify-content: center;
                cursor: pointer;

                &__text-area {
                    display: flex;
                    align-items: center;
                    justify-content: center;

                    &__plus-icon {
                        color: #0149ff;
                        font-family: sans-serif;
                        font-size: 24px;
                    }

                    &__text {
                        color: #0149ff;
                        font-family: sans-serif;
                        font-size: 18px;
                        text-decoration: underline;
                    }
                }
            }
        }
    }
</style>
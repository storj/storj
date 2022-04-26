// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pm-area">
        <div v-if="isAddModal" class="pm-area__add-modal">
            <div class="pm-area__add-modal__top">
                <h1 class="pm-area__add-modal__top__title" aria-roledescription="modal-title">Upgrade to Pro Account</h1>
                <div class="pm-area__add-modal__top__header">
                    <p class="pm-area__add-modal__top__header__sub-title">Add Payment Method</p>
                    <div class="pm-area__add-modal__top__header__choices">
                        <p class="pm-area__add-modal__top__header__choices__var" :class="{active: !isAddCard}" @click.stop="setIsAddToken">
                            STORJ Token
                        </p>
                        <p class="pm-area__add-modal__top__header__choices__var left-margin" :class="{active: isAddCard}" @click.stop="setIsAddCard">
                            Card
                        </p>
                    </div>
                </div>
            </div>
            <div v-if="isAddCard" class="pm-area__add-modal__card">
                <StripeCardInput
                    ref="stripeCardInput"
                    class="pm-area__add-modal__card__stripe"
                    :on-stripe-response-callback="addCardToDB"
                />
                <VButton
                    width="100%"
                    height="48px"
                    border-radius="32px"
                    label="Add Credit Card"
                    :on-press="onAddCardClick"
                />
                <p class="pm-area__add-modal__card__info">Pay as you go, no contract required.</p>
            </div>
            <div v-else class="pm-area__add-modal__tokens">
                <p class="pm-area__add-modal__tokens__banner">
                    Deposit STORJ Token to your account and receive a 10% bonus, or $10 for every $100.
                </p>
                <TokenDepositSelection
                    class="pm-area__add-modal__tokens__selection"
                    :payment-options="paymentOptions"
                    @onChangeTokenValue="onChangeTokenValue"
                />
                <VButton
                    width="100%"
                    height="48px"
                    border-radius="32px"
                    label="Continue to Coin Payments"
                    :on-press="onAddSTORJClick"
                />
                <div v-if="coinPaymentsCheckoutLink" class="pm-area__add-modal__tokens__checkout-container">
                    <a
                        class="pm-area__add-modal__tokens__checkout-container__link"
                        :href="coinPaymentsCheckoutLink"
                        target="_blank"
                        rel="noopener noreferrer"
                    >
                        Checkout
                    </a>
                </div>
                <p class="pm-area__add-modal__tokens__note">
                    Please Note: Your first deposit of $50 or more in STORJ Token is applied to your account after Coin
                    Payments verifies payment.
                </p>
                <p class="pm-area__add-modal__tokens__info">
                    After depositing STORJ Tokens, please contact
                    <a
                        class="pm-area__add-modal__tokens__info__link"
                        :href="limitsIncreaseRequestURL"
                        target="_blank"
                        rel="noopener noreferrer"
                    >
                        Support
                    </a>
                    to assist you for accessing your higher limits!
                </p>
            </div>
            <div class="pm-area__add-modal__bullets">
                <div class="pm-area__add-modal__bullets__left">
                    <h2 class="pm-area__add-modal__bullets__left__title">Pro Account includes:</h2>
                    <div class="pm-area__add-modal__bullets__left__item">
                        <CheckMarkIcon />
                        <p class="pm-area__add-modal__bullets__left__item__label">3 projects</p>
                    </div>
                    <div class="pm-area__add-modal__bullets__left__item">
                        <CheckMarkIcon />
                        <p class="pm-area__add-modal__bullets__left__item__label">100 buckets per project</p>
                    </div>
                    <div class="pm-area__add-modal__bullets__left__item">
                        <CheckMarkIcon />
                        <p class="pm-area__add-modal__bullets__left__item__label">25 TB storage per project</p>
                    </div>
                    <div class="pm-area__add-modal__bullets__left__item">
                        <CheckMarkIcon />
                        <p class="pm-area__add-modal__bullets__left__item__label">100 TB egress bandwidth per project</p>
                    </div>
                    <div class="pm-area__add-modal__bullets__left__item">
                        <CheckMarkIcon />
                        <p class="pm-area__add-modal__bullets__left__item__label">100 request per second rate limit</p>
                    </div>
                </div>
                <div class="pm-area__add-modal__bullets__right">
                    <h2 class="pm-area__add-modal__bullets__right__title">Storage price:</h2>
                    <div class="pm-area__add-modal__bullets__right__item">
                        <p class="pm-area__add-modal__bullets__right__item__price">$4</p>
                        <p class="pm-area__add-modal__bullets__right__item__label">TB / month</p>
                    </div>
                    <h2 class="pm-area__add-modal__bullets__right__title top-margin">Bandwidth price:</h2>
                    <div class="pm-area__add-modal__bullets__right__item">
                        <p class="pm-area__add-modal__bullets__right__item__price">$7</p>
                        <p class="pm-area__add-modal__bullets__right__item__label">TB</p>
                    </div>
                </div>
            </div>
            <div class="pm-area__add-modal__security">
                <LockImage />
                <p class="pm-area__add-modal__security__info">
                    Your information is secured with 128-bit SSL & AES-256 encryption.
                </p>
            </div>
            <div v-if="isLoading" class="pm-area__add-modal__blur">
                <VLoader
                    class="pm-area__add-modal__blur__loader"
                    width="30px"
                    height="30px"
                />
            </div>
            <div class="close-cross-container" @click="onClose">
                <CloseCrossIcon />
            </div>
        </div>
        <div v-else class="pm-area__success-modal">
            <BigCheckMarkIcon />
            <h2 class="pm-area__success-modal__title">Congratulations!</h2>
            <h2 class="pm-area__success-modal__sub-title">You've just upgraded to a Pro Account.</h2>
            <p class="pm-area__success-modal__info">
                Now you can have up to
                <b class="pm-area__success-modal__info__bold">75TB</b>
                of total storage and
                <b>300TB</b>
                of egress bandwidth per month. If you need more
                than this, please
                <a
                    class="pm-area__success-modal__info__link"
                    :href="limitsIncreaseRequestURL"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    contact us
                </a>
                .
            </p>
            <VButton
                width="100%"
                height="48px"
                border-radius="32px"
                label="Done"
                :on-press="onClose"
            />
            <div class="close-cross-container" @click="onClose">
                <CloseCrossIcon />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import StripeCardInput from '@/components/account/billing/paymentMethods/StripeCardInput.vue';
import TokenDepositSelection from '@/components/account/billing/paymentMethods/TokenDepositSelection.vue';
import VButton from '@/components/common/VButton.vue';
import VLoader from '@/components/common/VLoader.vue';

import LockImage from '@/../static/images/account/billing/greyLock.svg';
import CloseCrossIcon from '@/../static/images/common/closeCross.svg';
import CheckMarkIcon from '@/../static/images/common/greenRoundCheckmark.svg';
import BigCheckMarkIcon from '@/../static/images/common/greenRoundCheckmarkBig.svg';

import { RouteConfig } from '@/router';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { USER_ACTIONS } from '@/store/modules/users';
import { PaymentAmountOption } from '@/types/payments';
import { MetaUtils } from '@/utils/meta';

interface StripeForm {
    onSubmit(): Promise<void>;
}

// @vue/component
@Component({
    components: {
        StripeCardInput,
        VButton,
        CheckMarkIcon,
        LockImage,
        TokenDepositSelection,
        VLoader,
        CloseCrossIcon,
        BigCheckMarkIcon,
    },
})
export default class AddPaymentMethodModal extends Vue {
    @Prop({default: () => false})
    public readonly onClose: () => void;

    private readonly DEFAULT_TOKEN_DEPOSIT_VALUE = 50; // in dollars.
    private readonly MAX_TOKEN_AMOUNT = 1000000; // in dollars.
    private tokenDepositValue: number = this.DEFAULT_TOKEN_DEPOSIT_VALUE;

    public isAddModal = true;
    public isAddCard = true;
    public isLoading = false;
    public coinPaymentsCheckoutLink = '';

    public $refs!: {
        stripeCardInput: StripeCardInput & StripeForm;
    };

    /**
     * Set of default payment options.
     */
    public readonly paymentOptions: PaymentAmountOption[] = [
        new PaymentAmountOption(50, `USD $50`),
        new PaymentAmountOption(100, `USD $100`),
        new PaymentAmountOption(200, `USD $200`),
        new PaymentAmountOption(500, `USD $500`),
        new PaymentAmountOption(1000, `USD $1000`),
    ];

    /**
     * Provides card information to Stripe.
     */
    public async onAddCardClick(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        try {
            await this.$refs.stripeCardInput.onSubmit();
        } catch (error) {
            await this.$notify.error(error.message);
        }

        this.isLoading = false;
    }

    /**
     * Adds card after Stripe confirmation.
     *
     * @param token from Stripe
     */
    public async addCardToDB(token: string): Promise<void> {
        try {
            await this.$store.dispatch(PAYMENTS_ACTIONS.ADD_CREDIT_CARD, token);

            await this.$notify.success('Card successfully added');

            // We fetch User one more time to update their Paid Tier status.
            await this.$store.dispatch(USER_ACTIONS.GET);
            // We fetch Cards one more time to hide Paid Tier banner.
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_CREDIT_CARDS);

            if (this.$route.name === RouteConfig.ProjectDashboard.name) {
                await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, this.$store.getters.selectedProject.id);
            }
        } catch (error) {
            await this.$notify.error(error.message);
        }

        this.isLoading = false;
        this.isAddModal = false;
    }

    /**
     * onAddSTORJClick checks if amount is valid.
     * If so processes token payment and returns state to default.
     */
    public async onAddSTORJClick(): Promise<void> {
        if (this.isLoading) return;

        if (this.tokenDepositValue >= this.MAX_TOKEN_AMOUNT || this.tokenDepositValue === 0) {
            await this.$notify.error('Deposit amount must be more than $0 and less than $1000000');

            return;
        }

        this.isLoading = true;

        try {
            const tokenResponse = await this.$store.dispatch(PAYMENTS_ACTIONS.MAKE_TOKEN_DEPOSIT, this.tokenDepositValue * 100);
            await this.$notify.success(`Successfully created new deposit transaction! \nAddress:${tokenResponse.address} \nAmount:${tokenResponse.amount}`);
            const depositWindow = window.open(tokenResponse.link, '_blank');
            if (depositWindow) {
                depositWindow.focus();
            }

            this.coinPaymentsCheckoutLink = tokenResponse.link;

            if (this.$route.name === RouteConfig.Billing.name) {
                await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PAYMENTS_HISTORY);
            }
        } catch (error) {
            await this.$notify.error(error.message);
        }

        this.tokenDepositValue = this.DEFAULT_TOKEN_DEPOSIT_VALUE;
        this.isLoading = false;
    }

    /**
     * Sets modal state to add STORJ tokens.
     */
    public setIsAddToken(): void {
        this.isAddCard = false;
    }

    /**
     * Sets modal state to add credit card.
     */
    public setIsAddCard(): void {
        this.isAddCard = true;
    }

    /**
     * Event for changing token deposit value.
     */
    public onChangeTokenValue(value: number): void {
        this.tokenDepositValue = value;
    }

    /**
     * Returns project limits increase request url from config.
     */
    public get limitsIncreaseRequestURL(): string {
        return MetaUtils.getMetaContent('project-limits-increase-request-url');
    }
}
</script>

<style scoped lang="scss">
    .pm-area {
        position: fixed;
        top: 0;
        right: 0;
        left: 0;
        bottom: 0;
        z-index: 1000;
        background: rgba(27, 37, 51, 0.75);
        display: flex;
        align-items: center;
        justify-content: center;
        font-family: 'font_regular', sans-serif;

        &__add-modal {
            background: #fcfcfc;
            border-radius: 32px;
            box-shadow: 0 0 16px rgba(0, 0, 0, 0.04);
            width: 760px;
            position: relative;
            padding-top: 50px;

            &__top {
                padding: 0 50px;

                &__title {
                    font-family: 'font_bold', sans-serif;
                    font-size: 36px;
                    line-height: 44px;
                    color: #1b2533;
                    margin-bottom: 40px;
                }

                &__header {
                    display: flex;
                    align-items: center;
                    justify-content: space-between;
                    margin-bottom: 30px;

                    &__sub-title {
                        font-size: 18px;
                        line-height: 22px;
                        color: #000;
                    }

                    &__choices {
                        display: flex;
                        align-items: center;

                        &__var {
                            font-family: 'font_medium', sans-serif;
                            font-weight: 600;
                            font-size: 14px;
                            line-height: 18px;
                            color: #0149ff;
                            padding: 0 10px 5px 10px;
                            cursor: pointer;
                            border-bottom: 3px solid #fff;
                        }
                    }
                }
            }

            &__card {
                padding: 0 50px;
                margin-bottom: 20px;

                &__stripe {
                    margin: 20px 0;
                }

                &__info {
                    margin-top: 20px;
                    font-size: 12px;
                    line-height: 19px;
                    text-align: center;
                    color: #a8a8a8;
                }
            }

            &__tokens {
                padding: 0 50px;
                margin-bottom: 30px;

                &__banner {
                    font-size: 14px;
                    line-height: 20px;
                    color: #384761;
                    padding: 20px 35px;
                    background: #edf4fe;
                    border-radius: 8px;
                    margin-bottom: 25px;
                }

                &__selection {
                    margin-bottom: 25px;
                }

                &__checkout-container {
                    display: flex;
                    justify-content: center;
                    margin-top: 25px;

                    &__link {
                        font-size: 16px;
                        line-height: 20px;
                        color: #2683ff;
                    }
                }

                &__note {
                    font-size: 14px;
                    line-height: 20px;
                    color: #14142a;
                    margin: 25px 0;
                }

                &__info {
                    font-size: 14px;
                    line-height: 20px;
                    color: #14142a;

                    &__link {
                        font-family: 'font_medium', sans-serif;
                        text-decoration: underline !important;
                        text-underline-position: under;

                        &:visited {
                            color: #14142a;
                        }
                    }
                }
            }

            &__bullets {
                background: #f0f0f0;
                padding: 35px 50px 90px 50px;
                border-radius: 0 0 32px 32px;
                display: flex;

                &__left {
                    width: 50%;
                    border-right: 1px solid #ccc;

                    &__title {
                        font-family: 'font_bold', sans-serif;
                        font-size: 16px;
                        line-height: 26px;
                        color: #000;
                        margin-bottom: 5px;
                    }

                    &__item {
                        display: flex;
                        align-items: center;
                        margin-top: 12px;

                        &__label {
                            margin-left: 12px;
                            font-size: 14px;
                            line-height: 20px;
                            letter-spacing: 0.473506px;
                            color: #000;
                        }
                    }
                }

                &__right {
                    padding-left: 50px;

                    &__title {
                        font-family: 'font_bold', sans-serif;
                        font-size: 16px;
                        line-height: 26px;
                        color: #000;
                        margin-bottom: 5px;
                    }

                    &__item {
                        display: flex;
                        align-items: flex-start;
                        letter-spacing: 0.473506px;

                        &__price {
                            font-family: 'font_bold', sans-serif;
                            font-size: 42px;
                            color: #0149ff;
                        }

                        &__label {
                            font-family: 'font_medium', sans-serif;
                            font-size: 16px;
                            line-height: 20px;
                            color: #a9a9a9;
                            margin: 5px 0 0 5px;
                        }
                    }
                }
            }

            &__security {
                display: flex;
                align-items: center;
                justify-content: center;
                position: absolute;
                bottom: 0;
                left: 0;
                right: 0;
                background: #fff;
                border-radius: 32px;
                padding: 15px 0;

                &__info {
                    font-weight: 500;
                    font-size: 15px;
                    line-height: 18px;
                    color: #3f3f3f;
                    margin-left: 12px;
                }
            }

            &__blur {
                position: absolute;
                top: 0;
                bottom: 0;
                left: 0;
                right: 0;
                border-radius: 32px;
                z-index: 1;
                background-color: rgba(245, 246, 250, 0.5);
                display: flex;
                align-items: center;
                justify-content: center;
            }
        }

        &__success-modal {
            width: 480px;
            position: relative;
            padding: 50px;
            background: #fcfcfc;
            box-shadow: 0 0 16px rgba(0, 0, 0, 0.04);
            border-radius: 32px;

            &__title,
            &__sub-title {
                font-size: 36px;
                line-height: 54px;
                color: #000;
            }

            &__title {
                font-family: 'font_bold', sans-serif;
                margin-top: 20px;
            }

            &__sub-title {
                margin-top: 15px;
            }

            &__info {
                margin: 35px 0 48px 0;
                font-size: 18px;
                line-height: 32px;
                color: #000;

                &__bold {
                    font-family: 'font_bold', sans-serif;
                }

                &__link {
                    font-family: 'font_bold', sans-serif;
                    text-decoration: underline !important;
                    text-underline-position: under;

                    &:visited {
                        color: #000;
                    }
                }
            }
        }
    }

    .close-cross-container {
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

    .left-margin {
        margin-left: 20px;
    }

    .top-margin {
        margin-top: 25px;
    }

    .active {
        border-color: #0149ff;
    }

    ::v-deep .selected-container {
        width: calc(100% - 2px);
    }

    ::v-deep .custom-input {
        width: calc(100% - 68px);
    }

    ::v-deep .options-container,
    ::v-deep .payment-selection-blur {
        width: 100%;
    }

    @media screen and (max-height: 900px) {

        .pm-area {
            padding: 200px 0 20px 0;
            overflow-y: scroll;
        }
    }
</style>

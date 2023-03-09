// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payment-methods-container__card-container">
        <div class="payment-methods-container__card-container__info-area">
            <div class="payment-methods-container__card-container__info-area__card-logo">
                <component :is="cardIcon" />
            </div>
            <div class="payment-methods-container__card-container__info-area__info-container">
                <img src="@/../static/images/payments/cardStars.png" alt="Hidden card digits stars image">
                <h1 class="bold">{{ creditCard.last4 }}</h1>
            </div>
            <div class="payment-methods-container__card-container__info-area__expire-container">
                <h2 class="medium">Expires</h2>
                <h1 class="bold">{{ creditCard.expMonth }}/{{ creditCard.expYear }}</h1>
            </div>
        </div>
        <div class="payment-methods-container__card-container__button-area">
            <div v-if="creditCard.isDefault" class="payment-methods-container__card-container__default-button">
                <p class="payment-methods-container__card-container__default-button__label">Default</p>
            </div>
            <div v-else class="payment-methods-container__card-container__dots-container">
                <div @click.stop="toggleSelection">
                    <svg width="20" height="4" viewBox="0 0 20 4" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <rect width="4" height="4" rx="2" fill="#354049" />
                        <rect x="8" width="4" height="4" rx="2" fill="#354049" />
                        <rect x="16" width="4" height="4" rx="2" fill="#354049" />
                    </svg>
                </div>
                <CardDialog
                    v-if="creditCard.isSelected"
                    :card-id="creditCard.id"
                />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import Vue, { VueConstructor } from 'vue';
import { Component, Prop } from 'vue-property-decorator';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { CreditCard } from '@/types/payments';

import CardDialog from '@/components/account/billing/paymentMethods/CardDialog.vue';

import AmericanExpressIcon from '@/../static/images/payments/cardIcons/americanexpress.svg';
import DefaultIcon from '@/../static/images/payments/cardIcons/default.svg';
import DinersIcon from '@/../static/images/payments/cardIcons/dinersclub.svg';
import DiscoverIcon from '@/../static/images/payments/cardIcons/discover.svg';
import JCBIcon from '@/../static/images/payments/cardIcons/jcb.svg';
import MastercardIcon from '@/../static/images/payments/cardIcons/mastercard.svg';
import UnionPayIcon from '@/../static/images/payments/cardIcons/unionpay.svg';
import VisaIcon from '@/../static/images/payments/cardIcons/visa.svg';

const {
    TOGGLE_CARD_SELECTION,
} = PAYMENTS_ACTIONS;

// @vue/component
@Component({
    components: {
        CardDialog,
        JCBIcon,
        DinersIcon,
        MastercardIcon,
        AmericanExpressIcon,
        DiscoverIcon,
        UnionPayIcon,
        VisaIcon,
        DefaultIcon,
    },
})
export default class CardComponent extends Vue {
    @Prop({ default: () => new CreditCard() })
    private readonly creditCard: CreditCard;

    // TODO: move to CreditCard
    /**
     * Returns card logo depends of brand.
     */
    public get cardIcon(): VueConstructor<Vue> {
        switch (this.creditCard.brand) {
        case 'jcb':
            return JCBIcon;
        case 'diners':
            return DinersIcon;
        case 'mastercard':
            return MastercardIcon;
        case 'amex':
            return AmericanExpressIcon;
        case 'discover':
            return DiscoverIcon;
        case 'unionpay':
            return UnionPayIcon;
        case 'visa':
            return VisaIcon;
        default:
            return DefaultIcon;
        }
    }

    /**
     * Toggle card selection dialog.
     */
    public toggleSelection(): void {
        this.$store.dispatch(TOGGLE_CARD_SELECTION, this.creditCard.id);
    }
}
</script>

<style scoped lang="scss">
    .bold {
        font-family: 'font_bold', sans-serif;
        font-size: 16px;
        line-height: 21px;
        color: #61666b;
        margin-block-start: 0.5em;
        margin-block-end: 0.5em;
    }

    .medium {
        font-family: 'font_regular', sans-serif;
        font-size: 16px;
        line-height: 21px;
        color: #61666b;
        margin-right: 5px;
    }

    .payment-methods-container__card-container {
        width: calc(100% - 40px);
        margin-top: 12px;
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 20px;
        background-color: #f5f6fa;
        border-radius: 6px;

        &__info-area {
            width: 75%;
            display: flex;
            align-items: center;
            justify-content: flex-start;

            &__card-logo {
                display: flex;
                align-items: center;
                justify-content: center;
                height: 70px;
                width: 85px;
            }

            &__info-container {
                display: flex;
                flex-direction: row;
                align-items: center;
                justify-content: space-between;
                width: auto;
                min-width: 165px;
                margin-left: 15px;
            }

            &__expire-container {
                display: flex;
                flex-direction: row;
                align-items: center;
                justify-content: space-between;
                width: auto;
                margin-left: 30px;
            }
        }

        &__button-area {
            display: flex;
            justify-content: center;
            align-items: center;
        }

        &__default-button {
            width: 100px;
            height: 34px;
            border-radius: 6px;
            background-color: white;
            display: flex;
            justify-content: center;
            align-items: center;

            &__label {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 23px;
                color: #000;
            }
        }

        &__dots-container {
            width: 20px;
            height: 20px;
            display: flex;
            align-items: center;
            justify-content: center;
            margin-left: 10px;
            cursor: pointer;
            position: relative;
        }
    }

    .discover-svg-path {
        max-width: 80px;
    }
</style>

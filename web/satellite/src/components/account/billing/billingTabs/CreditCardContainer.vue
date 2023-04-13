// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payment-methods-container__card-container">
        <div class="payment-methods-container__card-container__info-area__card-logo">
            <component :is="cardIcon" />
        </div>
        <div class="payment-methods-container__card-container__info-area__card-text">
            Card Number
        </div>
        <div class="payment-methods-container__card-container__info-area__expiration-text">
            Exp. Date
        </div>

        <div class="payment-methods-container__card-container__info-area__info-container">
            <img src="@/../static/images/payments/cardStars.png" alt="Hidden card digits stars image" class="payment-methods-container__card-container__info-area__info-container__image">
            {{ props.creditCard.last4 }}
        </div>
        <div class="payment-methods-container__card-container__info-area__expire-container">
            {{ props.creditCard.expMonth }}/{{ props.creditCard.expYear }}
        </div>
        <div v-if="props.creditCard.isDefault" class="payment-methods-container__card-container__default-area">
            <div class="payment-methods-container__card-container__default-text">Default</div>
        </div>
        <div class="payment-methods-container__card-container__function-buttons">
            <div class="remove-button" @click="remove">
                <div class="remove-button__text">Remove</div>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { CreditCard } from '@/types/payments';

import AmericanExpressIcon from '@/../static/images/payments/cardIcons/americanexpress.svg';
import DefaultIcon from '@/../static/images/payments/cardIcons/default.svg';
import DinersIcon from '@/../static/images/payments/cardIcons/smalldinersclub.svg';
import DiscoverIcon from '@/../static/images/payments/cardIcons/discover.svg';
import JCBIcon from '@/../static/images/payments/cardIcons/smalljcb.svg';
import MastercardIcon from '@/../static/images/payments/cardIcons/smallmastercard.svg';
import UnionPayIcon from '@/../static/images/payments/cardIcons/smallunionpay.svg';
import VisaIcon from '@/../static/images/payments/cardIcons/visa.svg';

const icons = {
    'jcb': JCBIcon,
    'diners': DinersIcon,
    'mastercard': MastercardIcon,
    'amex': AmericanExpressIcon,
    'discover': DiscoverIcon,
    'unionpay': UnionPayIcon,
    'visa': VisaIcon,
};

const props = withDefaults(defineProps<{
    creditCard?: CreditCard;
}>(), {
    creditCard: () => new CreditCard(),
});

const emit = defineEmits(['edit', 'remove']);

const cardIcon = computed(() => {
    return icons[props.creditCard.brand] || DefaultIcon;
});

function edit(): void {
    emit('edit', props.creditCard);
}

function remove(): void {
    emit('remove', props.creditCard);
}
</script>

<style scoped lang="scss">
.medium {
    font-family: 'font_regular', sans-serif;
    font-size: 16px;
    line-height: 21px;
    color: #61666b;
    margin-right: 5px;
}

.remove-button:hover > .remove-button__text {
    background-color: #2683ff;
    color: white !important;
}

.remove-button {
    cursor: pointer;
    justify-content: center;
    align-items: center;
    gap: 8px;
    background: white;
    border: 1px solid var(--c-grey-3);
    box-shadow: 0 0 3px rgb(0 0 0 / 8%);
    border-radius: 6px;
    width: 70px;
    height: 30px;
    font-size: 13px;
    line-height: 23px;
    margin: 0 7px 0 0;
    white-space: nowrap;
    color: #354049;
    align-self: end;

    &:hover {
        background-color: #2683ff;
        color: white;
    }

    &__text {
        font-family: 'font_medium', sans-serif;
        line-height: 23px;
        white-space: nowrap;
        color: #354049 !important;
        margin: 4px 0 0 9px;
    }
}

.edit-button {
    cursor: pointer;
    justify-content: center;
    align-items: center;
    gap: 8px;
    width: 70px;
    height: 30px;
    background: white;
    border: 1px solid var(--c-grey-3);
    box-shadow: 0 0 3px rgb(0 0 0 / 8%);
    border-radius: 6px;
    font-family: 'font_medium', sans-serif;
    line-height: 23px;
    margin: 0;
    white-space: nowrap;
    color: #354049 !important;

    &__text {
        font-family: 'font_medium', sans-serif;
        line-height: 23px;
        white-space: nowrap;
        color: #354049 !important;
        font-size: 13px;
        margin: 4px 0 0 22px;
    }
}

.payment-methods-container__card-container {
    justify-content: space-between;
    border-radius: 6px;
    display: grid;
    grid-template-columns: 4fr 2fr;
    grid-template-rows: 1fr 0fr 1fr 1fr;
    height: 100%;
    font-family: 'font_regular', sans-serif;

    &__function-buttons {
        grid-column: 1;
        grid-row: 4;
        display: flex;
    }

    &__info-area {
        width: 75%;
        display: flex;
        align-items: center;
        justify-content: flex-start;

        &__card-logo {
            grid-column: 1;
            grid-row: 1;
        }

        &__card-text {
            grid-column: 1;
            grid-row: 2;
            font-family: 'font_bold', sans-serif;
            font-size: 12px;
            line-height: 18px;
            color: var(--c-grey-6);
        }

        &__expiration-text {
            grid-column: 2;
            grid-row: 2;
            font-family: 'font_bold', sans-serif;
            font-size: 12px;
            line-height: 18px;
            color: var(--c-grey-6);
        }

        &__last-four {
            grid-row: 3;
            grid-column: 1;
        }

        &__info-container {
            grid-row: 3;
            grid-column: 1;
            font-family: 'font_bold', sans-serif;
            font-size: 16px;
            line-height: 24px;
            color: #000;

            &__image {
                width: 50%;
            }
        }

        &__expire-container {
            grid-row: 3;
            grid-column: 2;
            font-family: 'font_bold', sans-serif;
            font-size: 16px;
            line-height: 24px;
            color: #000;
        }
    }

    &__default-area {
        grid-row: 1;
        grid-column: 2;
        width: 42px;
        height: 18px;
        background: var(--c-blue-1);
        border: 1px solid var(--c-blue-2);
        border-radius: 4px;
        justify-self: end;
        padding: 3px 8px;
    }

    &__default-text {
        font-family: 'font_bold', sans-serif;
        font-size: 12px;
        line-height: 20px;
        color: var(--c-blue-4);
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

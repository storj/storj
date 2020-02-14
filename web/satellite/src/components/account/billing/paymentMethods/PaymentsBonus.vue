// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payments-bonus-container">
        <LogoIcon v-if="anyCreditCards" />
        <GiftIcon v-else />
        <div class="payments-bonus-container__text-container" v-if="anyCreditCards">
            <p class="payments-bonus-container__text-container__main-text">
                Get free credits for paying in STORJ!
            </p>
            <p class="payments-bonus-container__text-container__additional-text">
                Deposit STORJ tokens to your account and receive a 10% bonus, or $10 for every $100.
            </p>
        </div>
        <div class="payments-bonus-container__text-container" v-else >
            <p class="payments-bonus-container__text-container__main-text">
                Receive 1TB of Free Storage & Bandwidth for the Production Release!
            </p>
            <p class="payments-bonus-container__text-container__additional-text">
                Add a credit card and get free credit. You will not be charged until the production release.
            </p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import GiftIcon from '@/../static/images/account/billing/Gift.svg';
import LogoIcon from '@/../static/images/navigation/logo.svg';

@Component({
    components: {
        GiftIcon,
        LogoIcon,
    },
})
export default class PaymentsBonus extends Vue {
    /**
     * Indicates if any credit card is attached to account.
     */
    @Prop({default: false})
    public readonly anyCreditCards: boolean;
}
</script>

<style scoped lang="scss">
    .payments-bonus-container {
        display: flex;
        justify-content: flex-start;
        align-items: center;
        width: calc(100% - 72px);
        padding: 30px 36px;
        background-color: #e9f3ff;
        border-radius: 6px;

        &__image {
            width: 77px;
            height: 77px;
        }

        &__text-container {
            margin-left: 35px;

            &__main-text {
                font-family: 'font_bold', sans-serif;
                font-size: 22px;
                line-height: 30px;
                margin: 0;
                color: #1e2d42;
            }

            &__additional-text {
                font-family: 'font_regular', sans-serif;
                font-size: 14px;
                line-height: 20px;
                margin: 12px 0 0 0;
                color: #717e92;
            }
        }
    }
</style>

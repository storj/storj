// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payout-container">
        <WalletIcon
            class="payout-container__image"
            alt="wallet image"
        />
        <div class="payout-container__wallet-address-section">
            <p class="payout-container__wallet-address-section__label">{{label}}</p>
            <p class="payout-container__wallet-address-section__bold-text">{{walletAddress}}</p>
        </div>
        <a
            class="payout-container__button"
            :href="'https://etherscan.io/address/' + walletAddress + '#tokentxns'"
            target="_blank"
            rel="noopener"
        >
            <b class="payout-container-button-label">View on Etherscan</b>
        </a>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import WalletIcon from '@/../static/images/wallet.svg';

@Component({
    components: {
        WalletIcon,
    },
})
export default class PayoutArea extends Vue {
    @Prop({default: ''})
    private readonly label: string;
    @Prop({default: ''})
    private readonly walletAddress: string;
}
</script>

<style scoped lang="scss">
    .payout-container {
        background-color: var(--block-background-color);
        padding: 40px;
        display: flex;
        justify-content: flex-start;
        align-items: center;
        border: 1px solid var(--block-border-color);
        border-radius: 12px;
        position: relative;

        &__image {
            margin-right: 40px;
        }

        &__wallet-address-section {
            height: 50px;
            display: flex;
            flex-direction: column;
            justify-content: space-between;

            &__label {
                font-size: 14px;
                color: var(--regular-text-color);
            }

            &__bold-text {
                font-family: 'font_bold', sans-serif;
                font-size: 18px;
                color: var(--regular-text-color);
                word-break: break-all;
            }
        }

        &__button {
            font-size: 14px;
            width: 168px;
            height: 44px;
            display: flex;
            align-items: center;
            justify-content: center;
            background-color: var(--button-background-color);
            border: 1px solid var(--block-border-color);
            border-radius: 12px;
            position: absolute;
            top: 42px;
            right: 40px;
            color: var(--regular-text-color);
            text-decoration: none;

            &:hover {
                background-color: #4d72b7;
                cursor: pointer;

                .payout-container-button-label {
                    color: #fff;
                }
            }
        }
    }

    @media screen and (max-width: 1000px) {

        .payout-container {
            flex-direction: column;
            justify-content: flex-start;
            align-items: flex-start;

            &__button {
                position: inherit;
                left: 0;
                margin: 20px 0 25px 0;
            }
        }
    }

    @media screen and (max-width: 500px) {

        p {
            margin: 0;
        }

        .payout-container {

            &__wallet-address-section {

                &__label {
                    margin: 20px 0 6px 0;
                }
            }

            &__button {
                margin: 10px 0 25px 0;
            }
        }
    }
</style>

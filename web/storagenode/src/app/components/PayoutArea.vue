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
            <p><b class="payout-container__wallet-address-section__bold-text">{{walletAddress}}</b></p>
        </div>
        <a :href="'https://etherscan.io/address/' + walletAddress + '#tokentxns'" target="_blank" rel="noopener">
            <div class="payout-container__button"><b class="payout-container-button-label">View on Etherscan</b></div>
        </a>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import WalletIcon from '@/../static/images/wallet.svg';

@Component({
    components: {
        WalletIcon
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
        background-color: #fff;
        padding: 40px;
        display: flex;
        justify-content: flex-start;
        align-items: center;
        border: 1px solid #eaeaea;
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
                color: #586c86;
                user-select: none;
            }

            &__bold-text {
                font-size: 18px;
                color: #535f77;
            }
        }

        &__button {
            font-size: 14px;
            width: 168px;
            height: 44px;
            display: flex;
            align-items: center;
            justify-content: center;
            background-color: #f4f6f9;
            border: 1px solid #e8e8e8;
            border-radius: 12px;
            position: absolute;
            top: 42px;
            right: 40px;
            color: #535f77;

            &:hover {
                background-color: #4d72b7;
                cursor: pointer;

                .payout-container-button-label {
                    color: #fff;
                }
            }
        }
    }
</style>

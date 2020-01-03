// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-balance-area">
        <div class="account-balance-area__title-area">
            <h1 class="account-balance-area__title-area__title">Account Balance</h1>
            <VInfo
                class="account-balance-area__title-area__info-button"
                bold-text="Prepaid STORJ token amount and any additional credits, minus current usage">
                <svg class="account-balance-area__title-area__info-button__image" width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <rect class="account-balance-svg-rect" x="0.5" y="0.5" width="19" height="19" rx="9.5" stroke="#AFB7C1"/>
                    <path class="account-balance-svg-path" d="M7 7.25177C7.00959 6.23527 7.28777 5.44177 7.83453 4.87129C8.38129 4.29043 9.1199 4 10.0504 4C10.952 4 11.6667 4.22819 12.1942 4.68458C12.7314 5.14097 13 5.79444 13 6.64498C13 7.03913 12.9376 7.38661 12.8129 7.68741C12.6882 7.98821 12.5396 8.24234 12.3669 8.44979C12.1942 8.65724 11.9592 8.90099 11.6619 9.18105C11.2686 9.54408 10.9712 9.876 10.7698 10.1768C10.5779 10.4672 10.482 10.8303 10.482 11.2659H9.04317C9.04317 10.851 9.10072 10.488 9.21583 10.1768C9.33094 9.86563 9.46523 9.6115 9.61871 9.41443C9.78177 9.20698 10.0024 8.96841 10.2806 8.69873C10.6067 8.37718 10.8465 8.09712 11 7.85856C11.1535 7.61999 11.2302 7.31919 11.2302 6.95615C11.2302 6.55163 11.1103 6.25082 10.8705 6.05375C10.6403 5.8463 10.3141 5.74257 9.89209 5.74257C9.45084 5.74257 9.10552 5.87223 8.85611 6.13154C8.60671 6.38048 8.47242 6.75389 8.45324 7.25177H7ZM9.73381 12.7595C10.0216 12.7595 10.2566 12.8633 10.4388 13.0707C10.6307 13.2782 10.7266 13.5427 10.7266 13.8642C10.7266 14.1961 10.6307 14.471 10.4388 14.6888C10.2566 14.8963 10.0216 15 9.73381 15C9.45564 15 9.22062 14.8911 9.02878 14.6733C8.84652 14.4554 8.7554 14.1858 8.7554 13.8642C8.7554 13.5427 8.84652 13.2782 9.02878 13.0707C9.22062 12.8633 9.45564 12.7595 9.73381 12.7595Z" fill="#354049"/>
                </svg>
            </VInfo>
        </div>
        <div class="account-balance-area__balance-area">
            <span class="account-balance-area__balance-area__balance">
                Balance
                <b
                    class="account-balance-area__balance-area__balance__bold-text"
                    :style="balanceStyle"
                >
                    {{balance}}
                </b>
            </span>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VInfo from '@/components/common/VInfo.vue';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';

const {
    GET_BALANCE,
} = PAYMENTS_ACTIONS;

@Component({
    components: {
        VInfo,
    },
})
export default class AccountBalance extends Vue {
    public mounted() {
        try {
            this.$store.dispatch(GET_BALANCE);
        } catch (error) {
            this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, error.message);
        }
    }

    public get balance(): string {
        return `USD $${this.$store.state.paymentsModule.balance / 100}`;
    }

    public get balanceStyle() {
        let color: string = '#000';

        if (this.$store.state.paymentsModule.balance < 0) {
            color = '#FF0000';
        }

        return { color };
    }

    public onEarnCredits(): void {
        return;
    }
}
</script>

<style scoped lang="scss">
    h1,
    span {
        margin: 0;
        color: #354049;
    }

    .account-balance-area {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 40px;
        margin: 55px 0 32px 0;
        background-color: #fff;
        border-radius: 8px;
        font-family: 'font_regular', sans-serif;

        &__title-area {
            display: flex;
            align-items: center;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 48px;
                margin-right: 13px;
                user-select: none;
            }

            &__info-button {
                max-height: 20px;
                cursor: pointer;
                margin-right: 10px;

                &:hover {

                    .account-balance-svg-path {
                        fill: #fff;
                    }

                    .account-balance-svg-rect {
                        fill: #2683ff;
                    }
                }
            }
        }

        &__balance-area {
            display: flex;
            align-items: center;

            &__balance {
                font-size: 18px;
                color: rgba(53, 64, 73, 0.5);

                &__bold-text {
                    color: #354049;
                }
            }
        }
    }

    /deep/ .info__message-box {
        background-image: url('../../../../../static/images/account/billing/MessageBox.png');
        background-repeat: no-repeat;
        min-height: 80px;
        min-width: 195px;
        top: 110%;
        left: -200%;
        padding: 0 20px 12px 20px;

        &__text {
            text-align: left;
            font-size: 13px;
            line-height: 17px;
            margin-top: 20px;

            &__bold-text {
                font-family: 'font_medium', sans-serif;
                color: #354049;
            }
        }
    }
</style>

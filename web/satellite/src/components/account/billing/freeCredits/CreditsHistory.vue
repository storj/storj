// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="credit-history">
        <div class="credit-history__back-area" @click.stop="onBackToBillingClick">
            <BackImage/>
            <p class="credit-history__back-area__title">Back to Billing</p>
        </div>
        <h1 class="credit-history__title">Free Credits</h1>
        <div class="credit-history__content">
            <h1 class="credit-history__content__sum">{{ remainingSum | centsToDollars }}</h1>
            <span class="credit-history__content__info">Available credits since last bill</span>
            <span class="credit-history__content__details">DETAILS</span>
            <SortingHeader/>
            <CreditsItem
                v-for="(item, index) in historyItems"
                :key="index"
                :credits-item="item"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import CreditsItem from '@/components/account/billing/freeCredits/CreditsItem.vue';
import SortingHeader from '@/components/account/billing/freeCredits/SortingHeader.vue';

import BackImage from '@/../static/images/account/billing/back.svg';

import { RouteConfig } from '@/router';
import { PaymentsHistoryItem, PaymentsHistoryItemType } from '@/types/payments';

@Component({
    components: {
        CreditsItem,
        BackImage,
        SortingHeader,
    },
})
export default class CreditsHistory extends Vue {
    /**
     * Returns list of free credit history items.
     */
    public get historyItems(): PaymentsHistoryItem[] {
        return this.$store.state.paymentsModule.paymentsHistory.filter((item: PaymentsHistoryItem) => {
            return item.type === PaymentsHistoryItemType.Coupon;
        });
    }

    /**
     * Returns remaining sum of items.
     */
    public get remainingSum(): number {
        const remainingAmounts: number[] = this.historyItems.map((item: PaymentsHistoryItem) => item.remaining);

        return remainingAmounts.reduce((accumulator, current) => accumulator + current);
    }

    /**
     * Replaces location to root billing route.
     */
    public onBackToBillingClick(): void {
        this.$router.push(RouteConfig.Billing.path);
    }
}
</script>

<style scoped lang="scss">
    p,
    h1 {
        margin: 0;
    }

    .credit-history {
        margin-top: 27px;
        padding: 0 0 80px 0;
        background-color: #f5f6fa;
        font-family: 'font_regular', sans-serif;

        &__back-area {
            display: flex;
            align-items: center;
            cursor: pointer;
            width: 184px;
            margin-bottom: 32px;

            &__title {
                font-family: 'font_medium', sans-serif;
                font-weight: 500;
                font-size: 16px;
                line-height: 21px;
                color: #768394;
                white-space: nowrap;
                margin-left: 15px;
            }

            &:hover {

                .credit-history__back-area__title {
                    color: #2683ff;
                }

                .back-button-svg-path {
                    fill: #2683ff;
                }
            }
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 22px;
            line-height: 27px;
            color: #384b65;
            margin-bottom: 20px;
        }

        &__content {
            background-color: #fff;
            padding: 40px 40px 30px 40px;
            border-radius: 8px;
            display: flex;
            flex-direction: column;
            align-items: flex-start;

            &__sum {
                font-family: 'font_bold', sans-serif;
                font-size: 36px;
                line-height: 53px;
                color: #384b65;
            }

            &__info {
                font-size: 16px;
                line-height: 24px;
                color: #909090;
                margin-bottom: 35px;
            }

            &__details {
                width: 100%;
                text-align: left;
                font-weight: 500;
                font-size: 16px;
                line-height: 23px;
                letter-spacing: 0.04em;
                color: #919191;
                padding-bottom: 22px;
                border-bottom: 1px solid #c7cdd2;
                margin-bottom: 75px;
            }
        }
    }

    ::-webkit-scrollbar,
    ::-webkit-scrollbar-track,
    ::-webkit-scrollbar-thumb {
        width: 0;
    }

    @media (max-height: 1000px) and (max-width: 1230px) {

        .credit-history {
            overflow-y: scroll;
            height: 65vh;
        }
    }
</style>

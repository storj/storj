// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="history-area">
        <div class="history-area__back-area" @click.stop="onBackToBillingClick">
            <BackImage/>
            <p class="history-area__back-area__title">Back to Billing</p>
        </div>
        <h1 class="history-area__title" v-if="isBillingHistory">Billing History</h1>
        <h1 class="history-area__title" v-else>Balance History</h1>
        <div class="history-area__content" v-if="historyItems.length > 0">
            <SortingHeader/>
            <PaymentsItem
                v-for="item in historyItems"
                :billing-item="item"
                :key="item.id"
            />
        </div>
        <h2 class="history-area__empty-state" v-else>No Items Yet</h2>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import PaymentsItem from '@/components/account/billing/depositAndBillingHistory/PaymentsItem.vue';
import SortingHeader from '@/components/account/billing/depositAndBillingHistory/SortingHeader.vue';

import BackImage from '@/../static/images/account/billing/back.svg';

import { RouteConfig } from '@/router';
import { PaymentsHistoryItem, PaymentsHistoryItemType } from '@/types/payments';

@Component({
    components: {
        PaymentsItem,
        SortingHeader,
        BackImage,
    },
})
export default class DetailedHistory extends Vue {
    /**
     * Returns list of history items depending on route name.
     */
    public get historyItems(): PaymentsHistoryItem[] {
        if (this.isBillingHistory) {
            return this.$store.state.paymentsModule.paymentsHistory.filter((item: PaymentsHistoryItem) => {
                return item.type === PaymentsHistoryItemType.Invoice || item.type === PaymentsHistoryItemType.Charge;
            });
        }

        return this.$store.state.paymentsModule.paymentsHistory.filter((item: PaymentsHistoryItem) => {
            return item.type === PaymentsHistoryItemType.Transaction || item.type === PaymentsHistoryItemType.DepositBonus;
        });
    }

    /**
     * Indicates if current route is billing history page.
     */
    public get isBillingHistory(): boolean {
        return this.$route.name === RouteConfig.BillingHistory.name;
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

    .history-area {
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

                .history-area__back-area__title {
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
            padding: 30px 40px 0 40px;
            border-radius: 8px;
        }

        &__empty-state {
            font-size: 40px;
            line-height: 46px;
            text-align: center;
            margin-top: 200px;
        }
    }

    ::-webkit-scrollbar,
    ::-webkit-scrollbar-track,
    ::-webkit-scrollbar-thumb {
        width: 0;
    }

    @media (max-height: 1000px) and (max-width: 1230px) {

        .history-area {
            overflow-y: scroll;
            height: 65vh;
        }
    }
</style>

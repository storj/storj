// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="billing-history-area">
        <div class="billing-history-area__title-area" @click="onBackToAccountClick">
            <div class="billing-history-area__title-area__back-button">
                <BackImage/>
            </div>
            <p class="billing-history-area__title-area__title">Back to Account</p>
        </div>
        <div class="billing-history-area__content">
            <h1 class="billing-history-area__content__title">Billing History</h1>
            <SortingHeader/>
            <BillingItem
                v-for="item in billingHistoryItems"
                :billing-item="item"
                :key="item.id"
            />
        </div>
<!--        <VPagination-->
<!--            v-if="totalPageCount > 1"-->
<!--            class="pagination-area"-->
<!--            :total-page-count="totalPageCount"-->
<!--            :on-page-click-callback="onPageClick"-->
<!--        />-->
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BillingItem from '@/components/account/billing/billingHistory/BillingItem.vue';
import SortingHeader from '@/components/account/billing/billingHistory/SortingHeader.vue';
import VPagination from '@/components/common/VPagination.vue';

import BackImage from '@/../static/images/account/billing/back.svg';

import { RouteConfig } from '@/router';
import { BillingHistoryItem } from '@/types/payments';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';

@Component({
    components: {
        BillingItem,
        SortingHeader,
        VPagination,
        BackImage,
    },
})
export default class BillingHistory extends Vue {
    /**
     * Lifecycle hook after initial render.
     */
    public mounted(): void {
        this.$segment.track(SegmentEvent.BILLING_HISTORY_VIEWED, {
            project_id: this.$store.getters.selectedProject.id,
            invoice_count: this.$store.state.paymentsModule.billingHistory.length,
        });
    }

    /**
     * Returns list of billing history listings.
     */
    public get billingHistoryItems(): BillingHistoryItem[] {
        return this.$store.state.paymentsModule.billingHistory;
    }

    /**
     * Replaces location to root billing route.
     */
    public onBackToAccountClick(): void {
        this.$router.push(RouteConfig.Billing.path);
    }
}
</script>

<style scoped lang="scss">
    p,
    h1 {
        margin: 0;
    }

    .billing-history-area {
        margin-top: 83px;
        padding: 0 0 80px 0;
        background-color: #f5f6fa;
        font-family: 'font_regular', sans-serif;

        &__title-area {
            display: flex;
            align-items: center;
            cursor: pointer;
            width: 184px;
            margin-bottom: 27px;

            &__back-button {
                display: flex;
                align-items: center;
                justify-content: center;
                background-color: #fff;
                width: 40px;
                height: 40px;
                border-radius: 78px;
                margin-right: 11px;
            }

            &__title {
                font-family: 'font_medium', sans-serif;
                font-weight: 500;
                font-size: 16px;
                line-height: 21px;
                color: #354049;
                white-space: nowrap;
            }

            &:hover {

                .billing-history-area__title-area__back-button {
                    background-color: #2683ff;

                    .back-button-svg-path {
                        fill: #fff;
                    }
                }
            }
        }

        &__content {
            background-color: #fff;
            padding: 32px 44px 34px 36px;
            border-radius: 8px;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 39px;
                color: #384b65;
                margin-bottom: 32px;
            }
        }
    }

    ::-webkit-scrollbar,
    ::-webkit-scrollbar-track,
    ::-webkit-scrollbar-thumb {
        width: 0;
    }

    @media (max-height: 1000px) and (max-width: 1230px) {

        .billing-history-area {
            overflow-y: scroll;
            height: 65vh;
        }
    }
</style>

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="billing-history-area">
        <div class="billing-history-area__title-area" @click="onBackToAccountClick">
            <div class="billing-history-area__title-area__back-button">
                <svg width="13" height="13" viewBox="0 0 13 13" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path class="billing-history-svg-path" fill-rule="evenodd" clip-rule="evenodd" d="M7.22572 0.300601C7.62652 0.701402 7.62652 1.35123 7.22572 1.75203L3.50406 5.47368H11.9737C12.5405 5.47368 13 5.93318 13 6.5C13 7.06682 12.5405 7.52632 11.9737 7.52632H3.50406L7.22572 11.248C7.62652 11.6488 7.62652 12.2986 7.22572 12.6994C6.82491 13.1002 6.17509 13.1002 5.77429 12.6994L0.300601 7.22571C-0.1002 6.82491 -0.1002 6.17509 0.300601 5.77428L5.77429 0.300601C6.17509 -0.1002 6.82491 -0.1002 7.22572 0.300601Z" fill="#384B65"/>
                </svg>
            </div>
            <p class="billing-history-area__title-area__title">Back to account</p>
        </div>
        <div class="billing-history-area__content">
            <h1 class="billing-history-area__content__title">Billing history</h1>
            <SortingHeader/>
            <BillingItem/>
            <BillingItem/>
            <BillingItem/>
            <BillingItem/>
            <BillingItem/>
            <BillingItem/>
        </div>
        <PaginationArea/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BillingItem from '@/components/account/billing/BillingItem.vue';
import PaginationArea from '@/components/account/billing/PaginationArea.vue';
import SortingHeader from '@/components/account/billing/SortingHeader.vue';

import { RouteConfig } from '@/router';

@Component({
    components: {
        PaginationArea,
        BillingItem,
        SortingHeader,
    },
})
export default class BillingHistory extends Vue {
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
        background-color: #F5F6FA;
        font-family: 'font_regular';

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
                background-color: #FFFFFF;
                width: 40px;
                height: 40px;
                border-radius: 78px;
                margin-right: 11px;
            }

            &__title {
                font-family: 'font_medium';
                font-weight: 500;
                font-size: 16px;
                line-height: 21px;
                color: #354049;
            }

            &:hover {

                .billing-history-area__title-area__back-button {
                    background-color: #2683FF;

                    .billing-history-svg-path {
                        fill: #FFFFFF;
                    }
                }
            }
        }

        &__content {
            background-color: #FFFFFF;
            padding: 32px 44px 34px 36px;
            border-radius: 8px;

            &__title {
                font-family: 'font_bold';
                font-size: 32px;
                line-height: 39px;
                color: #384B65;
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

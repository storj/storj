// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <BillingHistoryHeader />
        <VList 
            :data-set="testData"
            :item-component="billingHistoryStructure"
        />
    </div>
    <router-view />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { RouteConfig } from '@/router';
import VList from '@/components/common/VList.vue';
import VPagination from '@/components/common/VPagination.vue';
import BillingHistoryHeader from '@/components/account/billing/billingTabs/BillingHistorHeader.vue';
import BillingHistoryShape from '@/components/account/billing/billingTabs/BillingHistoryShape.vue';


// @vue/component
@Component({
    components: {
        VList,
        BillingHistoryHeader,
        BillingHistoryShape,
        VPagination
    },
})
export default class BillingArea extends Vue {

    private billingHistory;
    private testData = [ 
        { date: "11/5/2020", status: "Paid", amount: "$1,999,999", download: "button", isSelected: false },
        { date: "11/5/2020", status: "Paid", amount: "$1,999,999", download: "button", isSelected: false } 
    ];

     public mounted(): any {
       this.billingHistoryList();
       console.log(this.$store.state, 'state');
    }

     public get billingHistoryStructure(): any {
        return BillingHistoryShape;
    }

    public billingHistoryList(): any {

        this.billingHistory =  this.$store.state.paymentsModule.paymentsHistory;
        console.log(this.billingHistory, 'billingHistory');
    }
}
</script>

<style scoped lang="scss">
     .billing_history2 {
            position: relative;

            &__content {
                background-color: #fff;
                display: flex;
                flex-direction: column;
                justify-content: flex-start;
            }
     }
</style>
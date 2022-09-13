// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <BillingHistoryHeader />
        <VList 
            :data-set="historyItems"
            :item-component="billingHistoryStructure"
        />
        <router-view />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { PaymentsHistoryItem, PaymentsHistoryItemType } from '@/types/payments';

import VList from '@/components/common/VList.vue';
import BillingHistoryHeader from '@/components/account/billing/billingTabs/BillingHistoryHeader.vue';
import BillingHistoryShape from '@/components/account/billing/billingTabs/BillingHistoryShape.vue';

// @vue/component
@Component({
    components: {
        VList,
        BillingHistoryHeader,
    },
})

export default class BillingArea extends Vue {

    public get historyItems(): PaymentsHistoryItem[] {
        return this.$store.state.paymentsModule.paymentsHistory.filter((item: PaymentsHistoryItem) => {
            return item.type === PaymentsHistoryItemType.Invoice || item.type === PaymentsHistoryItemType.Charge;
        });
    }

    public get billingHistoryStructure() {
        return BillingHistoryShape;
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

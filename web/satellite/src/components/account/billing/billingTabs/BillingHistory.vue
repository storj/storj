// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="billing-history">
        <h1 class="billing-history__title">
            Billing History
        </h1>

        <v-table class="billing-history__table">
            <template #head>
                <BillingHistoryHeader />
            </template>
            <template #body>
                <BillingHistoryItem
                    v-for="item in historyItems"
                    :key="item.id"
                    :item="item"
                />
            </template>
        </v-table>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { PaymentsHistoryItem, PaymentsHistoryItemType } from '@/types/payments';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';

import BillingHistoryHeader from '@/components/account/billing/billingTabs/BillingHistoryHeader.vue';
import BillingHistoryItem from '@/components/account/billing/billingTabs/BillingHistoryItem.vue';
import VTable from '@/components/common/VTable.vue';

const {
    GET_PAYMENTS_HISTORY,
} = PAYMENTS_ACTIONS;

// @vue/component
@Component({
    components: {
        BillingHistoryItem,
        VTable,
        BillingHistoryHeader,
    },
})

export default class BillingArea extends Vue {

    mounted(): void {
        this.fetchHistory();
    }

    public async fetchHistory(): Promise<void> {
        try {
            await this.$store.dispatch(GET_PAYMENTS_HISTORY);
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    public get historyItems(): PaymentsHistoryItem[] {
        return this.$store.state.paymentsModule.paymentsHistory.filter((item: PaymentsHistoryItem) => {
            return item.type === PaymentsHistoryItemType.Invoice || item.type === PaymentsHistoryItemType.Charge;
        });
    }
}
</script>

<style scoped lang="scss">
    .billing-history {
        margin-top: 2rem;

        &__title {
            font-family: sans-serif;
            font-size: 1.5rem;
        }

        &__table {
            margin-top: 1.5rem;
        }
    }
</style>

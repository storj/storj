// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <section class="held-history-container">
        <div class="held-history-container__header">
            <p class="held-history-container__header__title">Held Amount History</p>
        </div>
        <div class="held-history-container__divider"></div>
        <HeldHistoryMonthlyBreakdownTable />
    </section>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeldHistoryMonthlyBreakdownTable from '@/app/components/payments/HeldHistoryMonthlyBreakdownTable.vue';

import { PAYOUT_ACTIONS } from '@/app/store/modules/payout';

@Component({
    components: {
        HeldHistoryMonthlyBreakdownTable,
    },
})
export default class HeldHistoryArea extends Vue {
    /**
     * Lifecycle hook before component render.
     * Fetches held history information.
     */
    public beforeMount(): void {
        this.$store.dispatch(PAYOUT_ACTIONS.GET_HELD_HISTORY);
    }
}
</script>

<style scoped lang="scss">
    .held-history-container {
        display: flex;
        flex-direction: column;
        padding: 28px 40px 10px 40px;
        background: var(--block-background-color);
        border: 1px solid var(--block-border-color);
        box-sizing: border-box;
        border-radius: 12px;
        margin: 12px 0 50px;

        &__header {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;

            &__title {
                font-family: 'font_medium', sans-serif;
                font-size: 18px;
                color: var(--regular-text-color);
            }
        }

        &__divider {
            width: 100%;
            height: 1px;
            margin-top: 18px;
            background-color: #eaeaea;
        }
    }
</style>

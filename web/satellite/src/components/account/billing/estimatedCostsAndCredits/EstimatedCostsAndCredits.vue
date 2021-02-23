// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="current-month-area">
        <h1 class="current-month-area__costs">{{ priceSummary | centsToDollars }}</h1>
        <span class="current-month-area__title">Estimated Charges for {{ chosenPeriod }}</span>
        <div class="current-month-area__content">
            <p class="current-month-area__content__title">DETAILS</p>
            <UsageAndChargesItem
                v-for="usageAndCharges in projectUsageAndCharges"
                :item="usageAndCharges"
                :key="usageAndCharges.projectId"
                class="item"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import UsageAndChargesItem from '@/components/account/billing/estimatedCostsAndCredits/UsageAndChargesItem.vue';
import VButton from '@/components/common/VButton.vue';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { ProjectUsageAndCharges } from '@/types/payments';
import { MONTHS_NAMES } from '@/utils/constants/date';

@Component({
    components: {
        VButton,
        UsageAndChargesItem,
    },
})
export default class EstimatedCostsAndCredits extends Vue {
    /**
     * Lifecycle hook after initial render.
     * Fetches current project usage rollup.
     */
    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_BALANCE);
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * projectUsageAndCharges is an array of all stored ProjectUsageAndCharges.
     */
    public get projectUsageAndCharges(): ProjectUsageAndCharges[] {
        return this.$store.state.paymentsModule.usageAndCharges;
    }

    /**
     * priceSummary returns price summary of usages for all the projects.
     */
    public get priceSummary(): number {
        return this.$store.state.paymentsModule.priceSummary;
    }

    /**
     * chosenPeriod returns billing period chosen by user.
     */
    public get chosenPeriod(): string {
        const dateFromStore = this.$store.state.paymentsModule.startDate;

        return `${MONTHS_NAMES[dateFromStore.getUTCMonth()]} ${dateFromStore.getUTCFullYear()}`;
    }
}
</script>

<style scoped lang="scss">
    h1,
    h2,
    p,
    span {
        margin: 0;
        color: #354049;
    }

    .current-month-area {
        margin-bottom: 32px;
        padding: 40px 40px 0 40px;
        background-color: #fff;
        border-radius: 8px;
        font-family: 'font_regular', sans-serif;

        &__costs {
            font-size: 36px;
            line-height: 53px;
            color: #384b65;
            font-family: 'font_medium', sans-serif;
        }

        &__title {
            font-size: 16px;
            line-height: 24px;
            color: #909090;
        }

        &__content {
            margin-top: 35px;

            &__title {
                font-size: 16px;
                line-height: 23px;
                letter-spacing: 0.04em;
                text-transform: uppercase;
                color: #919191;
                margin-bottom: 25px;
            }

            &__usage-charges {
                margin: 18px 0 0 0;
                background-color: #f5f6fa;
                border-radius: 12px;
                cursor: pointer;
            }
        }
    }

    .item {
        border-top: 1px solid #c7cdd2;
    }
</style>

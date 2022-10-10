// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="current-month-area">
        <VLoader v-if="isDataFetching" class="consts-loader" />
        <template v-else>
            <h1 class="current-month-area__costs">{{ priceSummary | centsToDollars }}</h1>
            <span class="current-month-area__title">Estimated Charges for {{ chosenPeriod }}</span>
            <p class="current-month-area__info">
                If you still have Storage and Bandwidth remaining in your free tier, you won’t be charged. This information
                is to help you estimate what charges would have been had you graduated to the paid tier.
            </p>
            <div class="current-month-area__content">
                <p class="current-month-area__content__title">DETAILS</p>
                <UsageAndChargesItem
                    v-for="usageAndCharges in projectUsageAndCharges"
                    :key="usageAndCharges.projectId"
                    :item="usageAndCharges"
                    class="item"
                />
            </div>
        </template>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { ProjectUsageAndCharges } from '@/types/payments';
import { MONTHS_NAMES } from '@/utils/constants/date';

import VLoader from '@/components/common/VLoader.vue';
import UsageAndChargesItem from '@/components/account/billing/estimatedCostsAndCredits/UsageAndChargesItem.vue';

// @vue/component
@Component({
    components: {
        UsageAndChargesItem,
        VLoader,
    },
})
export default class EstimatedCostsAndCredits extends Vue {
    public isDataFetching = true;

    /**
     * Lifecycle hook after initial render.
     * Fetches projects and usage rollup.
     */
    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch(PROJECTS_ACTIONS.FETCH);
        } catch (error) {
            this.isDataFetching = false;
            return;
        }

        try {
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);

            this.isDataFetching = false;
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
        padding: 40px 40px 0;
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

        &__info {
            font-size: 14px;
            line-height: 20px;
            color: #909090;
            margin: 15px 0 0;
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
                margin: 18px 0 0;
                background-color: #f5f6fa;
                border-radius: 12px;
                cursor: pointer;
            }
        }
    }

    .item {
        border-top: 1px solid #c7cdd2;
    }

    .consts-loader {
        padding-bottom: 40px;
    }
</style>

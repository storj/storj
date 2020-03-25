// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="current-month-area">
        <div class="current-month-area__title-area">
            <h1 class="current-month-area__title-area__title">Estimated Costs for This Billing Period</h1>
            <span class="current-month-area__title-area__costs">{{ chargesSummary | centsToDollars }}</span>
        </div>
        <div class="current-month-area__content">
            <h2 class="current-month-area__content__title">DETAILS</h2>
            <div class="current-month-area__content__usage-charges" @click="toggleUsageChargesPopup">
                <div class="current-month-area__content__usage-charges__head">
                    <div class="current-month-area__content__usage-charges__head__name-area">
                        <div class="current-month-area__content__usage-charges__head__name-area__image-container" v-if="usageCharges.length > 0">
                            <ArrowRightIcon v-if="!areUsageChargesShown"/>
                            <ArrowDownIcon v-if="areUsageChargesShown"/>
                        </div>
                        <span class="current-month-area__content__usage-charges__head__name-area__title">Usage Charges</span>
                    </div>
                    <span>Estimated total <span class="summary">{{ chargesSummary | centsToDollars }}</span></span>
                </div>
                <div class="current-month-area__content__usage-charges__content" v-if="areUsageChargesShown" @click.stop>
                    <UsageChargeItem
                        v-for="usageCharge in usageCharges"
                        :item="usageCharge"
                        :key="usageCharge.projectId"
                        class="item"
                    />
                </div>
            </div>
            <div class="current-month-area__content__credits-area">
                <div class="current-month-area__content__credits-area__title-area">
                    <span class="current-month-area__content__credits-area__title-area__title">Earned Credits</span>
                </div>
                <span
                    :style="{ color: balanceColor }"
                    class="current-month-area__content__credits-area__balance"
                >
                    {{ balance | centsToDollars }}
                </span>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import UsageChargeItem from '@/components/account/billing/estimatedCostsAndCredits/UsageChargeItem.vue';
import VButton from '@/components/common/VButton.vue';

import ArrowRightIcon from '@/../static/images/common/BlueArrowRight.svg';
import ArrowDownIcon from '@/../static/images/common/BlueExpand.svg';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { ProjectCharge } from '@/types/payments';

@Component({
    components: {
        VButton,
        UsageChargeItem,
        ArrowRightIcon,
        ArrowDownIcon,
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
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_CHARGES_CURRENT_ROLLUP);
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * areUsageChargesShown indicates if area with all projects is expanded.
     */
    private areUsageChargesShown: boolean = false;

    /**
     * usageCharges is an array of all stored ProjectCharges.
     */
    public get usageCharges(): ProjectCharge[] {
        return this.$store.state.paymentsModule.charges;
    }

    /**
     * chargesSummary returns summary of all projects.
     */
    public get chargesSummary(): number {
        if (!this.usageCharges.length) {
            return 0;
        }

        const usageItemSummaries = this.usageCharges.map(item => item.summary());

        return usageItemSummaries.reduce((accumulator, current) => accumulator + current);
    }

    /**
     * Returns balance from store in cents.
     */
    public get balance(): number {
        return this.$store.state.paymentsModule.balance;
    }

    /**
     * Returns balance color red if balance below zero and clack if not.
     */
    public get balanceColor(): string {
        return this.$store.state.paymentsModule.balance < 0 ? '#FF0000' : '#000';
    }

    /**
     * toggleUsageChargesPopup is used to open/close area with list of project charges.
     */
    public toggleUsageChargesPopup(): void {
        if (this.usageCharges.length === 0) {
            return;
        }

        this.areUsageChargesShown = !this.areUsageChargesShown;
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
        padding: 40px;
        background-color: #fff;
        border-radius: 8px;
        font-family: 'font_regular', sans-serif;

        &__title-area {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding-bottom: 40px;
            border-bottom: 1px solid rgba(169, 181, 193, 0.3);

            &__title,
            &__costs {
                font-size: 28px;
                line-height: 42px;
                font-family: 'font_bold', sans-serif;
                color: #354049;
            }
        }

        &__content {
            margin-top: 20px;

            &__title {
                font-size: 16px;
                line-height: 23px;
                letter-spacing: 0.04em;
                text-transform: uppercase;
                color: #919191;
            }

            &__usage-charges {
                position: relative;
                margin: 18px 0 0 0;
                background-color: #f5f6fa;
                border-radius: 12px;
                cursor: pointer;

                &__head {
                    display: flex;
                    justify-content: space-between;
                    align-items: center;
                    padding: 20px;

                    &__name-area {
                        display: flex;
                        align-items: center;

                        &__image-container {
                            max-width: 14px;
                            max-height: 14px;
                            width: 14px;
                            height: 14px;
                            display: flex;
                            align-items: center;
                            justify-content: center;
                            margin-right: 12px;
                        }
                    }
                }

                &__content {
                    cursor: default;
                    max-height: 228px;
                    overflow-y: auto;
                    padding: 0 20px;
                }
            }

            &__credits-area {
                display: flex;
                align-items: center;
                justify-content: space-between;
                padding: 20px;
                width: calc(100% - 40px);
                background-color: #f5f6fa;
                border-radius: 12px;
                margin-top: 20px;

                &__title-area {
                    display: flex;
                    align-items: center;

                    &__title {
                        font-size: 16px;
                        line-height: 21px;
                        color: #354049;
                    }
                }
            }
        }
    }

    .item {
        border-top: 1px solid rgba(169, 181, 193, 0.3);
    }

    .summary {
        user-select: text;
    }
</style>

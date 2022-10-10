// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="total-cost">
            <div class="total-cost__header-container">
                <h3 class="total-cost__header-container__title">Total Cost</h3>
                <div class="total-cost__header-container__date"><CalendarIcon />&nbsp;&nbsp;{{ currentDate }}</div>
            </div>
            <div class="total-cost__card-container">
                <div class="total-cost__card">
                    <EstimatedChargesIcon class="total-cost__card__main-icon" />
                    <p class="total-cost__card__money-text">{{ priceSummary | centsToDollars }}</p>
                    <p class="total-cost__card__label-text">
                        Total Estimated Charges
                        <img
                            src="@/../static/images/common/smallGreyWhiteInfo.png"
                            @mouseenter="showChargesTooltip = true"
                            @mouseleave="showChargesTooltip = false"
                        >
                    </p>
                    <div
                        v-if="showChargesTooltip"
                        class="total-cost__card__charges-tooltip"
                    >
                        <span class="total-cost__card__charges-tooltip__tooltip-text">If you still have Storage and Bandwidth remaining in your free tier, you won't be charged. This information is to help you estimate what the charges would have been had you graduated to the paid tier.</span>
                    </div>
                    <p
                        class="total-cost__card__link-text"
                        @click="routeToBillingHistory"
                    >
                        View Billing History →
                    </p>
                </div>
                <div class="total-cost__card">
                    <AvailableBalanceIcon class="total-cost__card__main-icon" />
                    <p class="total-cost__card__money-text">{{ balance.coins | centsToDollars }}</p>
                    <p class="total-cost__card__label-text">Available Balance</p>
                    <p
                        class="total-cost__card__link-text"
                        @click="routeToPaymentMethods"
                    >
                        View Payment Methods →
                    </p>
                </div>
            </div>
        </div>
        <div class="cost-by-project">
            <h3 class="cost-by-project__title">Cost by Project</h3>
            <div class="cost-by-project__buttons">
                <v-button
                    label="Edit Payment Method"
                    font-size="13px"
                    width="auto"
                    height="30px"
                    icon="lock"
                    :is-transparent="true"
                    class="cost-by-project__buttons__none-assigned"
                    :on-press="routeToPaymentMethods"
                />
                <v-button
                    label="See Payments"
                    font-size="13px"
                    width="auto"
                    height="30px"
                    icon="document"
                    :is-transparent="true"
                    class="cost-by-project__buttons__none-assigned"
                    :on-press="routeToBillingHistory"
                />
            </div>
            <div class="usage-charges-item-container__detailed-info-container__footer__buttons">
                <UsageAndChargesItem2
                    v-for="usageAndCharges in projectUsageAndCharges"
                    :key="usageAndCharges.projectId"
                    :item="usageAndCharges"
                    class="cost-by-project__item"
                />
            </div>
            <router-view />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { SHORT_MONTHS_NAMES } from '@/utils/constants/date';
import { AccountBalance , ProjectUsageAndCharges } from '@/types/payments';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

import UsageAndChargesItem2 from '@/components/account/billing/estimatedCostsAndCredits/UsageAndChargesItem2.vue';
import VButton from '@/components/common/VButton.vue';

import EstimatedChargesIcon from '@/../static/images/account/billing/totalEstimatedChargesIcon.svg';
import AvailableBalanceIcon from '@/../static/images/account/billing/availableBalanceIcon.svg';
import CalendarIcon from '@/../static/images/account/billing/calendar-icon.svg';

// @vue/component
@Component({
    components: {
        EstimatedChargesIcon,
        AvailableBalanceIcon,
        UsageAndChargesItem2,
        CalendarIcon,
        VButton,
    },
})
export default class BillingArea extends Vue {
    public availableBalance = 0;
    public showChargesTooltip = false;
    public isDataFetching = true;
    public currentDate = '';

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

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
            this.isDataFetching = false;
        }

        const rawDate = new Date();
        let currentYear = rawDate.getFullYear();
        this.currentDate = `${SHORT_MONTHS_NAMES[rawDate.getMonth()]} ${currentYear}`;
    }

    /**
     * Returns account balance from store.
     */
    public get balance(): AccountBalance {
        return this.$store.state.paymentsModule.balance;
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

    public routeToBillingHistory(): void {
        this.analytics.eventTriggered(AnalyticsEvent.SEE_PAYMENTS_CLICKED);
        this.$router.push(RouteConfig.Account.with(RouteConfig.Billing).with(RouteConfig.BillingHistory2).path);
    }

    public routeToPaymentMethods(): void {
        this.analytics.eventTriggered(AnalyticsEvent.EDIT_PAYMENT_METHOD_CLICKED);
        this.$router.push(RouteConfig.Account.with(RouteConfig.Billing).with(RouteConfig.BillingPaymentMethods).path);
    }

}
</script>

<style scoped lang="scss">
    .total-cost {
        font-family: sans-serif;
        margin: 20px 0;

        &__header-container {
            display: flex;
            justify-content: space-between;
            align-items: center;

            &__date {
                display: flex;
                justify-content: space-between;
                align-items: bottom;
                color: #56606d;
                font-weight: 700;
                font-family: sans-serif;
                border: 1px solid #d8dee3;
                border-radius: 5px;
                background-color: #fff;
                height: 15px;
                width: auto;
                padding: 10px;
                box-shadow: 0 0 3px rgb(0 0 0 / 8%);
            }
        }

        &__card-container {
            display: flex;
            justify-content: space-between;
            flex-wrap: wrap;
        }

        &__card {
            width: calc(50% - 50px);
            min-width: 188px;
            box-shadow: 0 0 20px rgb(0 0 0 / 4%);
            border-radius: 10px;
            background-color: #fff;
            padding: 20px;
            margin-top: 20px;
            display: flex;
            flex-direction: column;
            justify-content: left;
            position: relative;

            &__money-text {
                font-weight: 800;
                font-size: 32px;
                margin-top: 10px;
            }

            &__label-text {
                font-weight: 400;
                margin-top: 10px;
                min-width: 200px;
            }

            &__link-text {
                font-weight: medium;
                text-decoration: underline;
                margin-top: 10px;
                cursor: pointer;
            }

            &__main-icon {

                :deep(g) {
                    filter: none;
                }
            }

            &__charges-tooltip {
                top: 5px;
                left: 86px;

                @media screen and (max-width: 635px) {
                    top: 5px;
                    left: -21px;
                }

                position: absolute;
                background: #56606d;
                border-radius: 6px;
                width: 253px;
                color: #fff;
                display: flex;
                flex-direction: row;
                align-items: flex-start;
                padding: 8px;
                z-index: 1;
                transition: 250ms;

                &:after {
                    left: 50%;

                    @media screen and (max-width: 635px) {
                        left: 90%;
                    }

                    top: 100%;
                    content: '';
                    position: absolute;
                    bottom: 0;
                    width: 0;
                    height: 0;
                    border: 6px solid transparent;
                    border-top-color: #56606d;
                    border-bottom: 0;
                    margin-left: -20px;
                    margin-bottom: -20px;
                }

                &__tooltip-text {
                    text-align: center;
                    font-weight: 500;
                }
            }
        }
    }

    .cost-by-project {
        font-family: sans-serif;

        &__title {
            padding-bottom: 10px;
        }

        &__buttons {
            display: flex;
            align-self: center;
            flex-wrap: wrap;

            &__none-assigned {
                padding: 5px 10px;
                margin-right: 5px;
            }
        }
    }
</style>

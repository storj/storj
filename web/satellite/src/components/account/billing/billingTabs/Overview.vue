// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="total-cost">
            <h3 class="total-cost__title">Total Cost</h3>
            <div class="total-cost__card-container">
                <div class="total-cost__card">
                    <EstimatedChargesIcon class="total-cost__card__main-icon"/>
                    <p class="total-cost__card__money-text">${{ estimatedCharges }}</p>
                    <p class="total-cost__card__label-text">Total Estimated Charges 
                        <img 
                            src="@/../static/images/common/smallGreyWhiteInfo.png"
                            @mouseenter="showChargesTooltip = true"
                            @mouseleave="showChargesTooltip = false"
                        />
                    </p>
                    <div 
                        v-if="showChargesTooltip"
                        class="total-cost__card__charges-tooltip"
                    >
                        <span class="total-cost__card__charges-tooltip__tooltip-text">If you still have Storage and Bandwidth remaining in your free tier, you won't be charged. This information is to help you estimate what charges would have been had you graduated to the paid tier.</span>
                    </div>
                    <p class="total-cost__card__link-text">View Billing History →</p>
                </div>
                <div class="total-cost__card">
                    <AvailableBalanceIcon class="total-cost__card__main-icon"/>
                    <p class="total-cost__card__money-text">${{ availableBalance }}</p>
                    <p class="total-cost__card__label-text">Available Balance</p>
                    <p class="total-cost__card__link-text">View Payment Methods →</p>
                </div>
            </div>
        </div>
        <div class="cost-by-project">
            <h3 class="cost-by-project__title">Cost by Project</h3>

        </div>
        <router-view />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { RouteConfig } from '@/router';

import EstimatedChargesIcon from '@/../static/images/account/billing/totalEstimatedChargesIcon.svg';
import AvailableBalanceIcon from '@/../static/images/account/billing/availableBalanceIcon.svg';

// @vue/component
@Component({
    components: {
        EstimatedChargesIcon,
        AvailableBalanceIcon,
    },
})
export default class BillingArea extends Vue {
    public estimatedCharges: number = 0;
    public availableBalance: number = 0;
    public showChargesTooltip: boolean = false;


}
</script>

<style scoped lang="scss">
    .total-cost {
        font-family: sans-serif;
        margin: 20px 0;
        &__title {

        }

        &__card-container {
            display: flex;
            justify-content: space-between;
            flex-wrap: wrap;
        }

        &__card {
            width: calc(50% - 50px);
            min-width: 188px;
            box-shadow: 0px 0px 20px rgba(0, 0, 0, 0.04);
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
                font-weight: 500;
                text-decoration: underline;
                margin-top: 10px;
                cursor: pointer;
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
</style>
// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payouts-by-node">
        <div class="payouts-by-node__top-area">
            <div class="payouts-by-node__top-area__left-area">
                <div class="payouts-by-node__top-area__left-area__title-area">
                    <div class="payouts-by-node__top-area__left-area__title-area__arrow" @click="redirectToPayoutSummary">
                        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path fill-rule="evenodd" clip-rule="evenodd" d="M13.3398 0.554956C14.0797 1.2949 14.0797 2.49458 13.3398 3.23452L6.46904 10.1053H22.1053C23.1517 10.1053 24 10.9536 24 12C24 13.0464 23.1517 13.8947 22.1053 13.8947H6.46904L13.3398 20.7655C14.0797 21.5054 14.0797 22.7051 13.3398 23.445C12.5998 24.185 11.4002 24.185 10.6602 23.445L0.554956 13.3398C-0.184985 12.5998 -0.184985 11.4002 0.554956 10.6602L10.6602 0.554956C11.4002 -0.184985 12.5998 -0.184985 13.3398 0.554956Z" fill="#252A32"/>
                        </svg>
                    </div>
                    <h1 class="payouts-by-node__top-area__left-area__title-area__title">{{ nodeTitle }}</h1>
                </div>
                <p class="payouts-by-node__top-area__left-area__wallet">0xb64ef51c888972c908cfacf59b47c1afbc0ab8ac</p>
                <div class="payouts-by-node__top-area__left-area__links">
                    <v-link uri="#" label="View on Etherscan (L1 payouts)" />
                    <v-link uri="#" label="View on zkScan (L2 payouts)" />
                </div>
            </div>
            <info-block>
                <div class="payouts-by-node__top-area__balance" slot="body">
                    <div class="payouts-by-node__top-area__balance__item">
                        <h3 class="payouts-by-node__top-area__balance__item__label">Undistributed Balance</h3>
                        <h2 class="payouts-by-node__top-area__balance__item__value">$1,992.93</h2>
                    </div>
                    <div class="payouts-by-node__top-area__balance__divider"></div>
                    <div class="payouts-by-node__top-area__balance__item">
                        <h3 class="payouts-by-node__top-area__balance__item__label">Estimated Earnings (Apr)</h3>
                        <h2 class="payouts-by-node__top-area__balance__item__value">$1,992.93</h2>
                    </div>
                </div>
            </info-block>
        </div>
        <div class="payouts-by-node__content-area">
            <div class="payouts-by-node__content-area__dropdowns">
                <satellite-selection-dropdown />
                <payout-period-calendar-button :period="period" />
            </div>
            <section class="payouts-by-node__content-area__main-info">
                <payouts-by-node-table class="payouts-by-node__content-area__main-info__table"/>
                <div class="payouts-by-node__content-area__main-info__totals-area">
                    <info-block>
                        <div class="payouts-by-node__content-area__main-info__totals-area__item" slot="body">
                            <p class="payouts-by-node__content-area__main-info__totals-area__item__label">TOTAL PAID</p>
                            <p class="payouts-by-node__content-area__main-info__totals-area__item__value">$700.52</p>
                        </div>
                    </info-block>
                    <info-block>
                        <div class="payouts-by-node__content-area__main-info__totals-area__item" slot="body">
                            <p class="payouts-by-node__content-area__main-info__totals-area__item__label">TOTAL HELD</p>
                            <p class="payouts-by-node__content-area__main-info__totals-area__item__value">$130.52</p>
                        </div>
                    </info-block>
                    <info-block>
                        <div class="payouts-by-node__content-area__main-info__totals-area__item" slot="body">
                            <p class="payouts-by-node__content-area__main-info__totals-area__item__label">TOTAL EARNED</p>
                            <p class="payouts-by-node__content-area__main-info__totals-area__item__value">$830.52</p>
                        </div>
                    </info-block>
                    <info-block class="information">
                        <div class="payouts-by-node__content-area__main-info__totals-area__information" slot="body">
                            <h3 class="payouts-by-node__content-area__main-info__totals-area__information__title">Minimal threshold & distributed payout system</h3>
                            <p class="payouts-by-node__content-area__main-info__totals-area__information__description">Short description how minimal threshold system works.</p>
                            <a href="#"
                               class="payouts-by-node__content-area__main-info__totals-area__information__link"
                            >
                                Learn more
                            </a>
                        </div>
                    </info-block>
                </div>
            </section>
        </div>
        <section class="payouts-by-node__held-history">
            <h2 class="payouts-by-node__held-history__title">Held Amount History</h2>
            <held-history />
        </section>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import InfoBlock from '@/app/components/common/InfoBlock.vue';
import SatelliteSelectionDropdown from '@/app/components/common/SatelliteSelectionDropdown.vue';
import VLink from '@/app/components/common/VLink.vue';
import PayoutPeriodCalendarButton from '@/app/components/payouts/PayoutPeriodCalendarButton.vue';
import HeldHistory from '@/app/components/payouts/tables/heldHistory/HeldHistory.vue';
import PayoutsByNodeTable from '@/app/components/payouts/tables/payoutsByNode/PayoutsByNodeTable.vue';

import { UnauthorizedError } from '@/api';
import { Config as RouterConfig } from '@/app/router';

@Component({
    components: {
        HeldHistory,
        PayoutsByNodeTable,
        InfoBlock,
        VLink,
        PayoutPeriodCalendarButton,
        SatelliteSelectionDropdown,
    },
})
export default class PayoutsPage extends Vue {
    /**
     * Checks id path parameters and redirects if no provided.
     */
    public beforeMount(): void {
        if (!this.$route.params.id) {
            this.redirectToPayoutSummary();
        }
    }

    /**
     * payoutsSummary contains payouts summary from store.
     */
    public get nodeTitle(): string {
        const selectedNodeSummary = this.$store.state.payouts.summary.nodeSummary.find(summary => {
            return summary.nodeId === this.$route.params.id;
        });

        if (!selectedNodeSummary) return '';

        return selectedNodeSummary.title;
    }

    /**
     * period selected payout period from store.
     */
    public get period(): string {
        return this.$store.getters['payouts/periodString'];
    }

    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch('payouts/summary');
        } catch (error) {
            if (error instanceof UnauthorizedError) {
                // TODO: redirect to login screen.
            }

            // TODO: notify error
        }
    }

    public redirectToPayoutSummary(): void {
        this.$router.push(RouterConfig.PayoutsSummary);
    }
}
</script>

<style lang="scss" scoped>
    .payouts-by-node {
        box-sizing: border-box;
        padding: 60px;
        overflow-y: auto;
        height: calc(100vh - 60px);

        &__top-area {
            width: 100%;
            display: flex;
            align-items: flex-start;
            justify-content: space-between;

            &__left-area {
                width: 53%;
                margin-right: 36px;

                &__title-area {
                    display: flex;
                    align-items: center;
                    justify-content: flex-start;
                    margin-bottom: 36px;

                    &__arrow {
                        display: flex;
                        align-items: center;
                        justify-content: center;
                        width: 32px;
                        height: 32px;
                        max-width: 32px;
                        max-height: 32px;
                        cursor: pointer;
                        margin-right: 20px;
                    }

                    &__title {
                        font-family: 'font_bold', sans-serif;
                        font-size: 32px;
                        color: var(--c-title);
                        white-space: nowrap;
                        text-overflow: ellipsis;
                        position: relative;
                        overflow: hidden;
                        width: 100%;
                    }
                }

                &__wallet {
                    font-family: 'font_medium', sans-serif;
                    font-size: 16px;
                    color: var(--c-title);
                    margin-bottom: 16px;
                }

                &__links {
                    width: 100%;
                    display: flex;
                    align-items: center;
                    justify-content: flex-start;

                    & *:not(:first-of-type) {
                        margin-left: 20px;
                    }
                }
            }

            &__balance {
                display: flex;
                align-items: center;
                justify-content: space-between;

                &__item {
                    display: flex;
                    flex-direction: column;
                    align-items: flex-start;
                    justify-content: space-between;
                    max-width: 200px;

                    &__label {
                        font-size: 16px;
                        color: var(--c-gray);
                        font-family: 'font_medium', sans-serif;
                        margin-bottom: 10px;
                    }

                    &__value {
                        font-size: 22px;
                        font-family: 'font_bold', sans-serif;
                        color: var(--c-title);
                    }
                }

                &__divider {
                    height: 60px;
                    width: 1px;
                    background: var(--c-gray--light);
                }
            }
        }

        &__content-area {
            width: 100%;
            margin-top: 48px;

            &__dropdowns {
                width: 100%;
                display: flex;
                align-items: center;
                justify-content: space-between;

                & > *:first-of-type {
                    margin-right: 20px;
                }

                .calendar-button,
                .dropdown {
                    max-width: unset;
                }
            }

            &__main-info {
                display: flex;
                align-items: flex-start;
                justify-content: space-between;
                width: 100%;
                margin-top: 20px;

                &__table {
                    width: 75%;
                    min-width: 750px;
                }

                &__totals-area {
                    width: 23%;

                    &__item,
                    &__information {
                        display: flex;
                        flex-direction: column;
                        align-items: flex-start;
                        font-family: 'font_semiBold', sans-serif;

                        &__label {
                            font-size: 12px;
                            color: var(--c-gray);
                            margin-bottom: 10px;
                        }

                        &__value {
                            font-size: 18px;
                            color: var(--c-title);
                        }
                    }

                    &__information {
                        font-size: 14px;
                        color: var(--c-title);

                        &__title {
                            font-family: 'font_bold', sans-serif;
                            font-size: 16px;
                            margin-bottom: 8px;
                        }

                        &__description {
                            font-family: 'font_regular', sans-serif;
                            margin-bottom: 16px;
                        }

                        &__link {
                            text-decoration: none;
                            color: var(--c-primary);
                        }
                    }
                }
            }
        }

        &__held-history {
            width: 75%;
            margin-top: 40px;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 24px;
                margin-bottom: 20px;
                color: var(--c-title);
            }
        }
    }

    .info-block {
        margin-bottom: 20px;
        padding: 20px;

        &.information {
            background: #f8f8f9;
        }
    }
</style>

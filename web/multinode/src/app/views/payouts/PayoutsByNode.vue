// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payouts-by-node">
        <div class="payouts-by-node__top-area">
            <div class="payouts-by-node__top-area__left-area">
                <div class="payouts-by-node__top-area__left-area__title-area">
                    <div class="payouts-by-node__top-area__left-area__title-area__arrow" @click="redirectToPayoutSummary">
                        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path fill-rule="evenodd" clip-rule="evenodd" d="M13.3398 0.554956C14.0797 1.2949 14.0797 2.49458 13.3398 3.23452L6.46904 10.1053H22.1053C23.1517 10.1053 24 10.9536 24 12C24 13.0464 23.1517 13.8947 22.1053 13.8947H6.46904L13.3398 20.7655C14.0797 21.5054 14.0797 22.7051 13.3398 23.445C12.5998 24.185 11.4002 24.185 10.6602 23.445L0.554956 13.3398C-0.184985 12.5998 -0.184985 11.4002 0.554956 10.6602L10.6602 0.554956C11.4002 -0.184985 12.5998 -0.184985 13.3398 0.554956Z" fill="currentColor" />
                        </svg>
                    </div>
                    <h1 class="payouts-by-node__top-area__left-area__title-area__title">{{ nodeTitle }}</h1>
                </div>
                <!--                TODO: implement wallet info-->
                <!--                <p class="payouts-by-node__top-area__left-area__wallet">0xb64ef51c888972c908cfacf59b47c1afbc0ab8ac</p>-->
                <!--                <div class="payouts-by-node__top-area__left-area__links">-->
                <!--                    <v-link uri="#" label="View on Etherscan (L1 payouts)" />-->
                <!--                    <v-link uri="#" label="View on zkScan (L2 payouts)" />-->
                <!--                </div>-->
            </div>
            <info-block>
                <template #body>
                    <div class="payouts-by-node__top-area__balance">
                        <div class="payouts-by-node__top-area__balance__item">
                            <h3 class="payouts-by-node__top-area__balance__item__label">Undistributed Balance</h3>
                            <h2 class="payouts-by-node__top-area__balance__item__value">{{ Currency.dollarsFromCents(selectedNodePayouts.expectations.undistributed) }}</h2>
                        </div>
                        <div class="payouts-by-node__top-area__balance__divider" />
                        <div class="payouts-by-node__top-area__balance__item">
                            <h3 class="payouts-by-node__top-area__balance__item__label">Estimated Earnings (Apr)</h3>
                            <h2 class="payouts-by-node__top-area__balance__item__value">{{ Currency.dollarsFromCents(selectedNodePayouts.expectations.currentMonthEstimation) }}</h2>
                        </div>
                    </div>
                </template>
            </info-block>
        </div>
        <div class="payouts-by-node__content-area">
            <div class="payouts-by-node__content-area__dropdowns">
                <satellite-selection-dropdown />
                <payout-period-calendar-button :period="payoutsStore.periodString" />
            </div>
            <section class="payouts-by-node__content-area__main-info">
                <payouts-by-node-table class="payouts-by-node__content-area__main-info__table" :paystub="selectedNodePayouts.paystubForPeriod" />
                <div class="payouts-by-node__content-area__main-info__totals-area">
                    <info-block>
                        <template #body>
                            <div class="payouts-by-node__content-area__main-info__totals-area__item">
                                <p class="payouts-by-node__content-area__main-info__totals-area__item__label">TOTAL PAID</p>
                                <p class="payouts-by-node__content-area__main-info__totals-area__item__value">{{ Currency.dollarsFromCents(selectedNodePayouts.totalPaid) }}</p>
                            </div>
                        </template>
                    </info-block>
                    <info-block>
                        <template #body>
                            <div class="payouts-by-node__content-area__main-info__totals-area__item">
                                <p class="payouts-by-node__content-area__main-info__totals-area__item__label">TOTAL HELD</p>
                                <p class="payouts-by-node__content-area__main-info__totals-area__item__value">{{ Currency.dollarsFromCents(selectedNodePayouts.totalHeld) }}</p>
                            </div>
                        </template>
                    </info-block>
                    <info-block>
                        <template #body>
                            <div class="payouts-by-node__content-area__main-info__totals-area__item">
                                <p class="payouts-by-node__content-area__main-info__totals-area__item__label">TOTAL EARNED</p>
                                <p class="payouts-by-node__content-area__main-info__totals-area__item__value">{{ Currency.dollarsFromCents(selectedNodePayouts.totalEarned) }}</p>
                            </div>
                        </template>
                    </info-block>
                    <info-block class="information">
                        <template #body>
                            <div class="payouts-by-node__content-area__main-info__totals-area__information">
                                <h3 class="payouts-by-node__content-area__main-info__totals-area__information__title">Minimal threshold & distributed payout system</h3>
                                <p class="payouts-by-node__content-area__main-info__totals-area__information__description">Short description how minimal threshold system works.</p>
                                <!--                            TODO: consider moving link to config-->
                                <a
                                    href="https://forum.storj.io/t/minimum-threshold-for-storage-node-operator-payouts/11064"
                                    class="payouts-by-node__content-area__main-info__totals-area__information__link"
                                >
                                    Learn more
                                </a>
                            </div>
                        </template>
                    </info-block>
                </div>
            </section>
        </div>
        <section class="payouts-by-node__held-history">
            <h2 class="payouts-by-node__held-history__title">Held Amount History</h2>
            <held-history :held-history="selectedNodePayouts.heldHistory" />
        </section>
    </div>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router';
import { computed, onBeforeMount, onMounted } from 'vue';

import { UnauthorizedError } from '@/api';
import { Config as RouterConfig } from '@/app/router';
import { NodePayouts } from '@/payouts';
import { Currency } from '@/app/utils/currency';
import { usePayoutsStore } from '@/app/store/payoutsStore';
import { useNodesStore } from '@/app/store/nodesStore';

import InfoBlock from '@/app/components/common/InfoBlock.vue';
import SatelliteSelectionDropdown from '@/app/components/common/SatelliteSelectionDropdown.vue';
import PayoutPeriodCalendarButton from '@/app/components/payouts/PayoutPeriodCalendarButton.vue';
import HeldHistory from '@/app/components/payouts/tables/heldHistory/HeldHistory.vue';
import PayoutsByNodeTable from '@/app/components/payouts/tables/payoutsByNode/PayoutsByNodeTable.vue';

const route = useRoute();
const router = useRouter();

const payoutsStore = usePayoutsStore();
const nodesStore = useNodesStore();

const nodeId = computed<string>(() => route.params.id as string);

const nodeTitle = computed<string>(() => {
    const selectedNodeSummary = payoutsStore.state.summary.nodeSummary.find(summary => summary.nodeId === nodeId.value);
    if (!selectedNodeSummary) return nodeId.value;

    return selectedNodeSummary.title;
});

const selectedNodePayouts = computed<NodePayouts>(() => payoutsStore.state.selectedNodePayouts as NodePayouts);

function redirectToPayoutSummary(): void {
    router.push(RouterConfig.PayoutsSummary);
}

async function fetchNodePayouts(): Promise<void> {
    try {
        await Promise.all([
            payoutsStore.heldHistory(nodeId.value),
            payoutsStore.paystub(nodeId.value),
            payoutsStore.expectations(nodeId.value),
        ]);
    } catch (error) {
        if (error instanceof UnauthorizedError) {
            // TODO: redirect to login screen.
        }

        // TODO: notify error
    }
}

onBeforeMount(() => {
    if (!route.params.id) {
        redirectToPayoutSummary();
    }
});

onMounted(async () => {
    try {
        await payoutsStore.nodeTotals(nodeId.value);
    } catch (error) {
        if (error instanceof UnauthorizedError) {
            // TODO: redirect to login screen.
        }

        // TODO: notify error
    }

    await fetchNodePayouts();

    payoutsStore.$onAction(({ name, after }) => {
        if (name === 'setPayoutPeriod') {
            after(async () => {
                await fetchNodePayouts();
            });
        }
    });
    nodesStore.$onAction(({ name, after }) => {
        if (name === 'selectSatellite') {
            after(async () => {
                await fetchNodePayouts();
            });
        }
    });
});
</script>

<style lang="scss" scoped>
    .payouts-by-node {
        box-sizing: border-box;
        padding: 60px;
        overflow-y: auto;
        height: calc(100vh - 60px);
        background-color: var(--v-background-base);

        &__top-area {
            width: 100%;
            display: flex;
            align-items: flex-start;
            justify-content: space-between;

            &__left-area {
                width: 60%;
                min-width: 60%;
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
                        color: var(--v-header-base);
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
                    color: var(--v-header-base);
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
                        color: var(--v-text-base);
                        font-family: 'font_medium', sans-serif;
                        margin-bottom: 10px;
                    }

                    &__value {
                        font-size: 22px;
                        font-family: 'font_bold', sans-serif;
                        color: var(--v-header-base);
                    }
                }

                &__divider {
                    height: 60px;
                    width: 1px;
                    background: var(--v-border-base);
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
                            color: var(--v-text-base);
                            margin-bottom: 10px;
                        }

                        &__value {
                            font-size: 18px;
                            color: var(--v-header-base);
                        }
                    }

                    &__information {
                        font-size: 14px;
                        color: var(--v-header-base);

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
                color: var(--v-header-base);
            }
        }
    }

    .info-block {
        margin-bottom: 20px;
        padding: 20px;
        border: 1px solid var(--v-border-base);

        &.information {
            background: var(--v-background-base);
        }
    }
</style>

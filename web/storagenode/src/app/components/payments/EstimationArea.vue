// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="estimation-container">
        <div class="estimation-container__header">
            <p class="estimation-container__header__title">Info & Estimation, <span class="estimation-container__header__period">{{ currentPeriod }}</span></p>
            <div class="estimation-container__header__selection-area">
                <button
                    name="Select Current Period"
                    class="estimation-container__header__selection-area__item"
                    type="button"
                    :class="{ active: isCurrentPeriod }"
                    @click.stop="selectCurrentPeriod"
                >
                    <p class="estimation-container__header__selection-area__item__label long-text">
                        Current Period
                    </p>
                    <p class="estimation-container__header__selection-area__item__label short-text">
                        Current Per.
                    </p>
                </button>
                <EstimationPeriodDropdown
                    class="estimation-container__header__selection-area__item"
                    :class="{ active: !isCurrentPeriod }"
                />
            </div>
        </div>
        <div class="estimation-container__divider" />
        <div v-if="!isPayoutNoDataState" class="estimation-table-container">
            <div class="estimation-table-container__labels-area">
                <div class="column justify-start column-1">
                    <p class="estimation-table-container__labels-area__text">Name</p>
                </div>
                <div class="column justify-start column-2">
                    <p class="estimation-table-container__labels-area__text">Type</p>
                </div>
                <div class="column justify-start column-3">
                    <p v-if="isSatelliteSelected && !isHistoricalPeriod" class="estimation-table-container__labels-area__text">Price</p>
                </div>
                <div class="column justify-start column-4">
                    <p class="estimation-table-container__labels-area__text">Disk</p>
                </div>
                <div class="column justify-start column-5">
                    <p class="estimation-table-container__labels-area__text">Bandwidth</p>
                </div>
                <div class="column justify-end column-6">
                    <p class="estimation-table-container__labels-area__text">Payout</p>
                </div>
            </div>
            <div v-for="item in tableData" :key="item.name" class="estimation-table-container__info-area">
                <div class="column justify-start column-1">
                    <p class="estimation-table-container__info-area__text">{{ item.name }}</p>
                </div>
                <div class="column justify-start column-2">
                    <p class="estimation-table-container__info-area__text">{{ item.type }}</p>
                </div>
                <div class="column justify-start column-3">
                    <p v-if="isSatelliteSelected && !isHistoricalPeriod" class="estimation-table-container__info-area__text">{{ item.price }}</p>
                </div>
                <div class="column justify-start column-4">
                    <p class="estimation-table-container__info-area__text">{{ item.disk }}</p>
                </div>
                <div class="column justify-start column-5">
                    <p class="estimation-table-container__info-area__text">{{ item.bandwidth }}</p>
                </div>
                <div class="column justify-end column-6">
                    <p class="estimation-table-container__info-area__text">{{ centsToDollars(item.payout) }}</p>
                </div>
            </div>
            <div class="estimation-table-container__info-area">
                <div class="column justify-start column-1">
                    <p class="estimation-table-container__info-area__text">Gross Total</p>
                </div>
                <div class="column justify-start column-2" />
                <div class="column justify-start column-3" />
                <div class="column justify-start column-4" />
                <div class="column justify-start column-5" />
                <div class="column justify-end column-6">
                    <p class="estimation-table-container__info-area__text">{{ centsToDollars(grossTotal) }}</p>
                </div>
            </div>
            <div v-if="isHistoricalPeriod && totalPaystubForPeriod.surgePercent" class="estimation-table-container__total-area">
                <p class="estimation-table-container__total-area__text">Total + Surge {{ surgePercent }}</p>
                <p class="estimation-table-container__total-area__text">{{ centsToDollars(totalPaystubForPeriod.grossWithSurge) }}</p>
            </div>
            <div class="estimation-table-container__held-area">
                <p class="estimation-table-container__held-area__text">Held Back</p>
                <p class="estimation-table-container__held-area__text">-{{ centsToDollars(held) }}</p>
            </div>
            <div v-if="isHistoricalPeriod && disposed > 0" class="estimation-table-container__held-area">
                <p class="estimation-table-container__held-area__text">Held returned</p>
                <p class="estimation-table-container__held-area__text">{{ centsToDollars(disposed) }}</p>
            </div>
            <div class="estimation-table-container__net-total-area">
                <div class="column justify-start column-1">
                    <p class="estimation-table-container__net-total-area__text">NET TOTAL</p>
                </div>
                <div class="column justify-start column-2" />
                <div class="column justify-start column-3" />
                <div class="column justify-start column-4">
                    <p class="estimation-table-container__net-total-area__text">{{ totalDiskSpace + 'm' }}</p>
                </div>
                <div class="column justify-start column-5">
                    <p class="estimation-table-container__net-total-area__text">{{ totalBandwidth }}</p>
                </div>
                <div class="column justify-end column-6">
                    <p class="estimation-table-container__net-total-area__text">{{ centsToDollars(totalPayout) }}</p>
                </div>
            </div>
            <div v-if="!isCurrentPeriod && !isLastPeriodWithoutPaystub" class="estimation-table-container__distributed-area">
                <div class="estimation-table-container__distributed-area__left-area">
                    <p class="estimation-table-container__distributed-area__text">Distributed</p>
                    <div class="estimation-table-container__distributed-area__info-area">
                        <ChecksInfoIcon class="checks-area-image" alt="Info icon with question mark" @mouseenter="toggleTooltipVisibility" @mouseleave="toggleTooltipVisibility" />
                        <div v-show="isTooltipVisible" class="tooltip">
                            <div class="tooltip__text-area">
                                <p class="tooltip__text-area__text">If you see $0.00 as your distributed amount, you didn’t reach the minimum payout threshold. Your payout will be distributed along with one of the payouts in the upcoming payout cycles. If you see a distributed amount higher than expected, it means this month you were paid undistributed payouts from previous months in addition to this month’s payout.</p>
                            </div>
                            <div class="tooltip__footer" />
                        </div>
                    </div>
                </div>
                <p class="estimation-table-container__distributed-area__text">{{ centsToDollars(totalPaystubForPeriod.distributed) }}</p>
            </div>
        </div>
        <div v-if="isCurrentPeriod && !isFirstDayOfCurrentMonth" class="estimation-container__payout-area">
            <div class="estimation-container__payout-area__left-area">
                <p class="title-text">Estimated Payout</p>
                <p class="additional-text">At the end of the month if the load keeps the same for the rest of the month.</p>
            </div>
            <div class="estimation-container__payout-area__right-area">
                <p class="title-text">{{ centsToDollars(estimation.currentMonthExpectations) }}</p>
            </div>
        </div>
        <div v-if="isPayoutNoDataState" class="no-data-container">
            <img class="no-data-container__image" src="@/../static/images/payments/NoData.png" alt="no data">
            <p class="no-data-container__title">No data to display</p>
            <p class="no-data-container__additional-text">Please note, historical data about payouts does not update immediately, it may take some time.</p>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { monthNames, PayoutInfoRange } from '@/app/types/payout';
import { Size } from '@/private/memory/size';
import {
    EstimatedPayout,
    PayoutPeriod,
    SatellitePricingModel,
    TotalPaystubForPeriod,
} from '@/storagenode/payouts/payouts';
import { centsToDollars } from '@/app/utils/payout';
import { useNodeStore } from '@/app/store/modules/nodeStore';
import { usePayoutStore } from '@/app/store/modules/payoutStore';
import { useAppStore } from '@/app/store/modules/appStore';

import EstimationPeriodDropdown from '@/app/components/payments/EstimationPeriodDropdown.vue';

import ChecksInfoIcon from '@/../static/images/checksInfo.svg';

const nodeStore = useNodeStore();
const payoutStore = usePayoutStore();
const appStore = useAppStore();

/**
 * Describes table row data item.
 */
class EstimationTableRow {
    public constructor(
        public name: string = '',
        public type: string = '',
        public price: string = '0',
        public disk: string = '',
        public bandwidth: string = '',
        public payout: number = 0,
    ) {}
}

const now = ref<Date>(new Date());
const isTooltipVisible = ref<boolean>(false);

const currentPeriod = computed<string>(() => {
    const start: PayoutPeriod | null = payoutStore.state.periodRange.start;
    const end: PayoutPeriod = payoutStore.state.periodRange.end;

    return start && start.period !== end.period ?
        `${monthNames[start.month].slice(0, 3)} ${start.year} - ${monthNames[end.month].slice(0, 3)} ${end.year}`
        : `${monthNames[end.month].slice(0, 3)} ${end.year}`;
});

const isCurrentPeriod = computed<boolean>(() => {
    const end = payoutStore.state.periodRange.end;
    const isCurrentMonthSelected = end.year === now.value.getUTCFullYear() && end.month === now.value.getUTCMonth();

    return !payoutStore.state.periodRange.start && isCurrentMonthSelected;
});

const isLastPeriodWithoutPaystub = computed<boolean>(() => {
    const joinedAt: Date = nodeStore.state.selectedSatellite.joinDate;
    const isNodeStartedBeforeCurrentPeriod =
        joinedAt.getTime() < new Date(now.value.getUTCFullYear(), now.value.getUTCMonth(), 1, 0, 0, 1).getTime();

    if (!isNodeStartedBeforeCurrentPeriod) {
        return false;
    }

    const lastMonthDate = new Date();
    lastMonthDate.setMonth(lastMonthDate.getUTCMonth() - 1);

    const selectedPeriod: PayoutInfoRange = payoutStore.state.periodRange;
    const lastMonthPayoutPeriod = new PayoutPeriod(lastMonthDate.getUTCFullYear(), lastMonthDate.getUTCMonth());
    const isLastPeriodSelected: boolean = !selectedPeriod.start && selectedPeriod.end.period === lastMonthPayoutPeriod.period;
    const isPaystubAvailable: boolean = payoutStore.state.payoutPeriods.map(e => e.period).includes(lastMonthPayoutPeriod.period);

    return isLastPeriodSelected && !isPaystubAvailable;
});

const isSatelliteSelected = computed<boolean>(() => {
    return !!nodeStore.state.selectedSatellite.id;
});

const surgePercent = computed<string>(() => {
    return !payoutStore.state.periodRange.start ? `(${totalPaystubForPeriod.value.surgePercent}%)` : '';
});

const isPayoutNoDataState = computed<boolean>(() => {
    return appStore.state.isNoPayoutData;
});

const totalPaystubForPeriod = computed<TotalPaystubForPeriod>(() => {
    return payoutStore.state.totalPaystubForPeriod as TotalPaystubForPeriod;
});

const estimation = computed<EstimatedPayout>(() => {
    return payoutStore.state.estimation;
});

const pricing = computed<SatellitePricingModel>(() => {
    return payoutStore.state.pricingModel;
});

const held = computed<number>(() => {
    if (isHistoricalPeriod.value) {
        return totalPaystubForPeriod.value.held;
    }

    return estimatedHeld.value;
});

const disposed = computed<number>(() => {
    return totalPaystubForPeriod.value.disposed;
});

const isHistoricalPeriod = computed<boolean>(() => {
    return !isCurrentPeriod.value && !isLastPeriodWithoutPaystub.value;
});

const totalPayout = computed<number>(() => {
    if (isHistoricalPeriod.value) {
        return totalPaystubForPeriod.value.paid;
    }

    return grossTotal.value - estimatedHeld.value;
});

const grossTotal = computed<number>(() => {
    if (isHistoricalPeriod.value) {
        return totalPaystubForPeriod.value.paidWithoutSurge;
    }

    return isLastPeriodWithoutPaystub.value ? estimation.value.previousMonth.payout + held.value : estimation.value.currentMonth.payout + held.value;
});

const totalDiskSpace = computed<string>(() => {
    if (isHistoricalPeriod.value) {
        return Size.toBase10String(totalPaystubForPeriod.value.usageAtRest);
    }

    return Size.toBase10String(currentDiskSpace.value);
});

const totalBandwidth = computed<string>(() => {
    if (isHistoricalPeriod.value) {
        return Size.toBase10String(
            totalPaystubForPeriod.value.usageGet +
            totalPaystubForPeriod.value.usageGetRepair +
            totalPaystubForPeriod.value.usageGetAudit,
        );
    }

    return Size.toBase10String((currentBandwidthAuditAndRepair.value + currentBandwidthDownload.value));
});

const currentBandwidthAuditAndRepair = computed<number>(() => {
    return isLastPeriodWithoutPaystub.value ? estimation.value.previousMonth.egressRepairAudit : estimation.value.currentMonth.egressRepairAudit;
});

const currentBandwidthDownload = computed<number>(() => {
    return isLastPeriodWithoutPaystub.value ? estimation.value.previousMonth.egressBandwidth : estimation.value.currentMonth.egressBandwidth;
});

const currentDiskSpace = computed<number>(() => {
    return isLastPeriodWithoutPaystub.value ? estimation.value.previousMonth.diskSpace : estimation.value.currentMonth.diskSpace;
});

const tableData = computed<EstimationTableRow[]>(() => {
    if (isHistoricalPeriod.value) {
        return [
            new EstimationTableRow('Download', 'Egress', ``, '--', Size.toBase10String(totalPaystubForPeriod.value.usageGet), totalPaystubForPeriod.value.compGet),
            new EstimationTableRow('Repair & Audit', 'Egress', ``, '--', Size.toBase10String(totalPaystubForPeriod.value.usageGetRepair + totalPaystubForPeriod.value.usageGetAudit), totalPaystubForPeriod.value.compGetRepair + totalPaystubForPeriod.value.compGetAudit),
            new EstimationTableRow('Disk Average Month', 'Storage', ``, Size.toBase10String(totalPaystubForPeriod.value.usageAtRest) + 'm', '--', totalPaystubForPeriod.value.compAtRest),
        ];
    }

    const estimatedPayout = isLastPeriodWithoutPaystub.value ? estimation.value.previousMonth : estimation.value.currentMonth;

    return [
        new EstimationTableRow(
            'Download',
            'Egress',
            `$${pricing.value.egressBandwidth} / TB`,
            '--',
            Size.toBase10String(estimatedPayout.egressBandwidth),
            estimatedPayout.egressBandwidthPayout,
        ),
        new EstimationTableRow(
            'Repair & Audit',
            'Egress',
            `$${pricing.value.repairBandwidth} / TB`,
            '--',
            Size.toBase10String(estimatedPayout.egressRepairAudit),
            estimatedPayout.egressRepairAuditPayout,
        ),
        new EstimationTableRow(
            'Disk Average Month',
            'Storage',
            `$${pricing.value.diskSpace} / TBm`,
            Size.toBase10String(estimatedPayout.diskSpace) + 'm',
            '--',
            estimatedPayout.diskSpacePayout,
        ),
    ];
});

const isFirstDayOfCurrentMonth = computed<boolean>(() => {
    return now.value.getUTCDate() === 1;
});

const estimatedHeld = computed<number>(() => {
    return isLastPeriodWithoutPaystub.value ?
        estimation.value.previousMonth.held :
        estimation.value.currentMonth.held;
});

function toggleTooltipVisibility(): void {
    isTooltipVisible.value = !isTooltipVisible.value;
}

function selectCurrentPeriod(): void {
    const nowDate = new Date();

    appStore.setNoPayoutData(false);
    payoutStore.setPeriodsRange(new PayoutInfoRange(null, new PayoutPeriod(nowDate.getUTCFullYear(), nowDate.getUTCMonth())));
}
</script>

<style scoped lang="scss">
    .estimation-container {
        display: flex;
        flex-direction: column;
        padding: 28px 40px;
        background: var(--block-background-color);
        border: 1px solid var(--block-border-color);
        box-sizing: border-box;
        border-radius: 12px;
        font-family: 'font_regular', sans-serif;

        &__header {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;
            height: 40px;

            &__title {
                font-weight: 500;
                font-size: 18px;
                color: var(--regular-text-color);
            }

            &__period {
                color: #909bad;
            }

            &__selection-area {
                display: flex;
                align-items: center;
                justify-content: flex-end;
                height: 100%;

                &__item {
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    cursor: pointer;
                    height: 100%;
                    padding: 0 20px;
                    border-bottom: 3px solid transparent;
                    z-index: 102;

                    &__label {
                        text-align: center;
                        font-size: 16px;
                        color: var(--regular-text-color);
                    }

                    &.active {
                        border-bottom: 3px solid var(--navigation-link-color);

                        &__label {
                            font-size: 16px;
                            color: var(--regular-text-color);
                        }
                    }
                }
            }
        }

        &__total-held,
        &__payout-area {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;
            padding: 0 16px;
            width: calc(100% - 32px);
            height: 87px;

            &__left-area {
                display: flex;
                flex-direction: column;
                align-items: flex-start;
            }

            &__right-area {
                display: flex;
                flex-direction: column;
                align-items: flex-end;
            }
        }

        &__payout-area {
            height: auto;
            margin-top: 29px;
        }

        &__total-held {
            border-bottom: 1px solid #eaeaea;
        }

        &__divider {
            width: 100%;
            height: 1px;
            background-color: #eaeaea;
        }
    }

    .title-text {
        font-family: 'font_bold', sans-serif;
        font-size: 16px;
        line-height: 20px;
        color: var(--regular-text-color);
    }

    .additional-text {
        font-size: 13px;
        line-height: 17px;
        color: #b5bdcb;
    }

    .estimation-table-container {
        font-family: 'font_regular', sans-serif;

        &__labels-area {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: center;
            margin-top: 17px;
            padding: 0 16px;
            width: calc(100% - 32px);
            height: 36px;
            background: var(--table-header-color);

            &__text {
                font-weight: 500;
                font-size: 14px;
                color: #909bad;
            }
        }

        &__info-area {
            padding: 0 16px;
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: center;
            height: 56px;
            border-bottom: 1px solid #a9b5c1;

            &__text {
                font-size: 14px;
                color: var(--regular-text-color);
            }
        }

        &__held-area {
            padding: 0 16px;
            width: calc(100% - 32px);
            height: 56px;
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;
            border-bottom: 1px solid #a9b5c1;

            &__text {
                font-family: 'font_regular', sans-serif;
                font-size: 14px;
                color: var(--regular-text-color);
            }
        }

        &__net-total-area,
        &__total-area,
        &__distributed-area {
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 0 16px;
            width: calc(100% - 32px);
            height: 56px;

            &__text {
                font-family: 'font_bold', sans-serif;
                font-size: 14px;
                color: var(--regular-text-color);
            }
        }

        &__net-total-area,
        &__distributed-area {
            background-color: var(--estimation-table-total-container-color);
        }

        &__net-total-area {
            border-bottom: 1px solid #a9b5c1;
        }

        &__total-area {
            align-items: center;
            justify-content: space-between;
            border-bottom: 1px solid #a9b5c1;

            &__text {
                font-family: 'font_regular', sans-serif;
            }
        }

        &__distributed-area {
            justify-content: space-between;
            font-family: 'font_regular', sans-serif;

            &__info-area {
                position: relative;
                margin-left: 10px;
                width: 18px;
                height: 18px;
            }

            &__left-area {
                display: flex;
                align-items: center;
                justify-content: center;
            }
        }
    }

    .short-text {
        display: none;
    }

    .column {
        display: flex;
        flex-direction: row;
        align-items: center;
    }

    .justify-start {
        justify-content: flex-start;
    }

    .justify-end {
        justify-content: flex-end;
    }

    .column-1 {
        width: 26.9%;
    }

    .column-2 {
        width: 14.3%;
    }

    .column-3 {
        width: 13%;
    }

    .column-4 {
        width: 18.2%;
    }

    .column-5 {
        width: 18.9%;
    }

    .column-6 {
        width: 8.7%;
    }

    .no-data-container {
        display: flex;
        flex-direction: column;
        justify-content: center;
        align-items: center;
        padding: 50px 0 80px;
        font-family: 'font_regular', sans-serif;
        color: var(--regular-text-color);

        &__image {
            width: 248px;
            height: 252px;
            margin-bottom: 40px;
        }

        &__title {
            font-size: 26px;
        }

        &__additional-text {
            font-size: 16px;
            max-width: 500px;
            text-align: center;
            margin-top: 16px;
        }
    }

    .tooltip {
        position: absolute;
        bottom: 35px;
        left: 50%;
        transform: translate(-50%);
        height: auto;
        box-shadow: 0 2px 48px var(--tooltip-shadow-color);
        border-radius: 12px;
        background: var(--tooltip-background-color);

        &__text-area {
            padding: 15px 11px;
            width: 360px;
            font-family: 'font_regular', sans-serif;
            font-size: 11px;
            line-height: 17px;
            color: var(--regular-text-color);
            text-align: center;
        }

        &__footer {
            position: absolute;
            left: 50%;
            transform: translate(-50%);
            width: 0;
            height: 0;
            border-style: solid;
            border-width: 11.5px 11.5px 0;
            border-color: var(--tooltip-background-color) transparent transparent transparent;
        }
    }

    .checks-area-image {
        cursor: pointer;

        rect {
            fill: var(--info-icon-background);
        }

        :deep(path) {
            fill: var(--info-icon-letter);
        }
    }

    @media screen and (width <= 870px) {

        .estimation-container {

            &__header {
                flex-direction: column;
                align-items: flex-start;
                height: auto;

                &__selection-area {
                    width: 100%;
                    height: 41px;
                    margin: 20px 0;

                    &__item {
                        width: calc(50% - 40px);
                        border-bottom: 3px solid #eaeaea;
                    }
                }
            }

            &__divider {
                display: none;
            }
        }
    }

    @media screen and (width <= 640px) {

        .estimation-container {
            padding: 28px 20px;
        }

        .column-2,
        .column-3,
        .column-4,
        .column-5 {
            display: none;
        }

        .column-1 {
            width: 70%;
        }

        .column-6 {
            width: 30%;
        }
    }

    @media screen and (width <= 505px) {

        .short-text {
            display: inline-block;
            font-size: 14px;
        }

        .long-text {
            display: none;
        }
    }

    @media screen and (width <= 430px) {

        .estimation-container__header__period {
            display: block;
            margin-top: 8px;
        }
    }
</style>

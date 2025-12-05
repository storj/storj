// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payout-period-calendar">
        <div class="payout-period-calendar__header">
            <div class="payout-period-calendar__header__year-selection">
                <button name="Decrement year" class="payout-period-calendar__header__year-selection__prev" type="button" @click.prevent="decrementYear">
                    <GrayArrowLeftIcon />
                </button>
                <p class="payout-period-calendar__header__year-selection__year">{{ displayedYear }}</p>
                <button name="Increment year" class="payout-period-calendar__header__year-selection__next" type="button" @click.prevent="incrementYear">
                    <GrayArrowLeftIcon />
                </button>
            </div>
            <button name="Select All Time" class="payout-period-calendar__header__all-time" type="button" @click.prevent="selectAllTime">All time</button>
        </div>
        <div class="payout-period-calendar__months-area">
            <button
                v-for="item in currentDisplayedMonths"
                :key="item.name"
                :name="`Select year ${item.year} month ${item.name}`"
                class="month-item"
                type="button"
                :class="{ selected: item.selected, disabled: !item.active }"
                @click.prevent="checkMonth(item)"
            >
                <p class="month-item__label">{{ item.name }}</p>
            </button>
        </div>
        <div class="payout-period-calendar__footer-area">
            <p class="payout-period-calendar__footer-area__period">{{ period }}</p>
            <button name="Submit" class="payout-period-calendar__footer-area__ok-button" type="submit" @click.prevent="submit">OK</button>
        </div>
    </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';

import {
    MonthButton,
    monthNames,
    PayoutInfoRange,
    StoredMonthsByYear,
} from '@/app/types/payout';
import { PayoutPeriod } from '@/storagenode/payouts/payouts';
import { usePayoutStore } from '@/app/store/modules/payoutStore';
import { useAppStore } from '@/app/store/modules/appStore';
import { useNodeStore } from '@/app/store/modules/nodeStore';

import GrayArrowLeftIcon from '@/../static/images/payments/GrayArrowLeft.svg';

const payoutStore = usePayoutStore();
const appStore = useAppStore();
const nodeStore = useNodeStore();

const now = ref<Date>(new Date());
const currentDisplayedMonths = ref<MonthButton[]>([]);
const displayedYear = ref<number>(now.value.getUTCFullYear());
const period = ref<string>('');
const displayedMonths = ref<StoredMonthsByYear>({});
const firstSelectedMonth = ref<MonthButton | null>(null);
const secondSelectedMonth = ref<MonthButton | null>(null);

async function submit(): Promise<void> {
    if (!firstSelectedMonth.value) {
        close();

        return;
    }

    if (secondSelectedMonth.value) {
        payoutStore.setPeriodsRange(
            new PayoutInfoRange(
                new PayoutPeriod(firstSelectedMonth.value.year, firstSelectedMonth.value.index),
                new PayoutPeriod(secondSelectedMonth.value.year, secondSelectedMonth.value.index),
            ),
        );
    } else {
        payoutStore.setPeriodsRange(
            new PayoutInfoRange(
                null,
                new PayoutPeriod(firstSelectedMonth.value.year, firstSelectedMonth.value.index),
            ),
        );
    }

    try {
        await payoutStore.fetchPayoutInfo(nodeStore.state.selectedSatellite.id);
        appStore.setNoPayoutData(false);
    } catch (error) {
        const lastMonthDate = new Date();
        lastMonthDate.setMonth(lastMonthDate.getUTCMonth() - 1);

        const selectedPeriod: PayoutInfoRange = payoutStore.state.periodRange;
        const lastMonthPayoutPeriod = new PayoutPeriod(lastMonthDate.getUTCFullYear(), lastMonthDate.getUTCMonth());
        const isLastPeriodSelected: boolean = !selectedPeriod.start && selectedPeriod.end.period === lastMonthPayoutPeriod.period;

        if (!isLastPeriodSelected) {
            appStore.setNoPayoutData(true);
            console.error(error);
        }
    }

    close();
}

function updatePeriod(): void {
    if (!firstSelectedMonth.value) {
        period.value = '';

        return;
    }

    period.value = secondSelectedMonth.value ?
        `${firstSelectedMonth.value.name}, ${firstSelectedMonth.value.year} - ${secondSelectedMonth.value.name}, ${secondSelectedMonth.value.year}`
        : `${monthNames[firstSelectedMonth.value.index]}, ${firstSelectedMonth.value.year}`;
}

function selectAllTime(): void {
    const nodeStartedAt = nodeStore.state.selectedSatellite.joinDate;

    if (nodeStartedAt.getUTCMonth() === now.value.getUTCMonth() && nodeStartedAt.getUTCFullYear() === now.value.getUTCFullYear()) {
        return;
    }

    firstSelectedMonth.value = new MonthButton(nodeStartedAt.getUTCFullYear(), nodeStartedAt.getUTCMonth());
    secondSelectedMonth.value = now.value.getUTCMonth() === 0 ?
        new MonthButton(now.value.getUTCFullYear() - 1, 11)
        : new MonthButton(now.value.getUTCFullYear(), now.value.getUTCMonth() - 1);

    if (
        firstSelectedMonth.value.year === secondSelectedMonth.value.year
        && firstSelectedMonth.value.index === secondSelectedMonth.value.index
    ) {
        secondSelectedMonth.value = null;
        checkMonth(firstSelectedMonth.value);
    }

    updateMonthsSelection(true);
    updatePeriod();
}

function checkMonth(month: MonthButton): void {
    if (firstSelectedMonth.value && secondSelectedMonth.value) {
        updateMonthsSelection(false);
        firstSelectedMonth.value = secondSelectedMonth.value = null;

        if (month.active) {
            firstSelectedMonth.value = month;
            month.selected = true;
        }

        updatePeriod();

        return;
    }

    if (!month.active) return;

    if (!firstSelectedMonth.value) {
        firstSelectedMonth.value = month;
        month.selected = true;
        updatePeriod();

        return;
    }

    if (firstSelectedMonth.value === month) {
        firstSelectedMonth.value = null;
        month.selected = false;
        updatePeriod();

        return;
    }

    secondSelectedMonth.value = month;
    if ((secondSelectedMonth.value && firstSelectedMonth.value) && new Date(secondSelectedMonth.value.year, secondSelectedMonth.value.index) < new Date(firstSelectedMonth.value.year, firstSelectedMonth.value.index)) {
        [secondSelectedMonth.value, firstSelectedMonth.value] = [firstSelectedMonth.value, secondSelectedMonth.value];
    }

    updatePeriod();
    updateMonthsSelection(true);
}

function incrementYear(): void {
    if (displayedYear.value === now.value.getUTCFullYear()) return;

    displayedYear.value += 1;
    populateMonths(displayedYear.value);
    currentDisplayedMonths.value = displayedMonths.value[displayedYear.value];
}

function decrementYear(): void {
    if (displayedYear.value === nodeStore.state.selectedSatellite.joinDate.getUTCFullYear()) return;

    displayedYear.value -= 1;
    populateMonths(displayedYear.value);
    currentDisplayedMonths.value = displayedMonths.value[displayedYear.value];
}

function updateMonthsSelection(value: boolean): void {
    if (!firstSelectedMonth.value) return;

    if (!secondSelectedMonth.value) {
        const selectedMonth = displayedMonths.value[firstSelectedMonth.value.year].find(month => {
            if (firstSelectedMonth.value) {
                return month.index === firstSelectedMonth.value.index;
            }
        });

        if (selectedMonth) {
            selectedMonth.selected = value;
        }

        return;
    }

    for (let i = firstSelectedMonth.value.year; i <= secondSelectedMonth.value.year; i++) {
        if (!displayedMonths.value[i]) {
            populateMonths(i);
        }

        displayedMonths.value[i].forEach(month => {
            const date = new Date(month.year, month.index);

            if (
                (secondSelectedMonth.value && firstSelectedMonth.value)
                && new Date(firstSelectedMonth.value.year, firstSelectedMonth.value.index) <= date
                && date <= new Date(secondSelectedMonth.value.year, secondSelectedMonth.value.index)
            ) {
                month.selected = value;
            }
        });
    }
}

function populateMonths(year: number): void {
    if (displayedMonths.value[year]) {
        currentDisplayedMonths.value = displayedMonths.value[year];

        return;
    }

    const months: MonthButton[] = [];
    const availablePeriods: string[] = payoutStore.state.payoutPeriods.map(payoutPeriod => payoutPeriod.period);
    const lastMonthDate = new Date();
    lastMonthDate.setMonth(lastMonthDate.getUTCMonth() - 1);

    // Creates month entities and adds them to list.
    for (let i = 0; i < 12; i++) {
        const period = `${year}-${i < 9 ? '0' + (i + 1) : (i + 1)}`;

        const isLastMonth: boolean = lastMonthDate.getUTCFullYear() === year && lastMonthDate.getUTCMonth() === i;
        const isLastMonthActive: boolean =
            isLastMonth && nodeStore.state.selectedSatellite.joinDate.getTime() < new Date(
                now.value.getUTCFullYear(), now.value.getUTCMonth(), 1, 0, 0, 1,
            ).getTime();

        const isMonthActive: boolean = availablePeriods.includes(period);

        months.push(new MonthButton(year, i, isMonthActive || isLastMonthActive, false));
    }

    displayedMonths.value[year] = months;
}

function close(): void {
    setTimeout(() => appStore.togglePayoutCalendar(false), 0);
}

onMounted(() => {
    populateMonths(displayedYear.value);
    currentDisplayedMonths.value = displayedMonths.value[displayedYear.value];
});
</script>

<style scoped lang="scss">
    .payout-period-calendar {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: flex-start;
        width: 170px;
        height: 215px;
        background: var(--block-background-color);
        box-shadow: 0 10px 25px rgb(175 183 193 / 10%);
        border-radius: 5px;
        padding: 24px;
        font-family: 'font_regular', sans-serif;
        cursor: default;

        &__header {
            display: flex;
            flex-direction: row;
            justify-content: space-between;
            align-items: center;
            height: 20px;
            width: 100%;

            &__year-selection {
                display: flex;
                flex-direction: row;
                align-items: center;
                justify-content: flex-start;

                &__prev {
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    cursor: pointer;
                    margin-right: 20px;
                    height: 20px;
                    width: 15px;
                }

                &__year {
                    font-family: 'font_bold', sans-serif;
                    font-size: 15px;
                    line-height: 18px;
                    color: var(--regular-text-color);
                }

                &__next {
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    cursor: pointer;
                    transform: rotate(180deg);
                    margin-left: 20px;
                    height: 20px;
                    width: 15px;
                }
            }

            &__all-time {
                font-size: 12px;
                line-height: 18px;
                color: var(--navigation-link-color);
                cursor: pointer;
            }
        }

        &__months-area {
            margin: 13px 0;
            display: grid;
            grid-template-columns: 52px 52px 52px;
            gap: 8px;
        }

        &__footer-area {
            display: flex;
            flex-direction: row;
            justify-content: space-between;
            align-items: center;
            height: 20px;
            width: 100%;
            margin-top: 7px;

            &__period {
                font-size: 13px;
                color: var(--regular-text-color);
            }

            &__ok-button {
                font-family: 'font_bold', sans-serif;
                font-size: 16px;
                line-height: 23px;
                color: var(--navigation-link-color);
                cursor: pointer;
            }
        }
    }

    .month-item {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 52px;
        height: 30px;
        background: var(--month-active-background-color);
        border-radius: 10px;
        cursor: pointer;

        &__label {
            font-size: 12px;
            line-height: 18px;
            color: var(--regular-text-color);
        }
    }

    .disabled {
        background: var(--month-disabled-background-color);
        cursor: default;

        .month-item__label {
            color: var(--month-disabled-label-color) !important;
        }
    }

    .selected {
        background: var(--navigation-link-color);

        .month-item__label {
            color: white !important;
        }
    }

    .arrow-icon :deep(path) {
        fill: var(--year-selection-arrow-color);
    }
</style>

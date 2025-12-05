// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payout-period-calendar">
        <div class="payout-period-calendar__header">
            <div class="payout-period-calendar__header__year-selection">
                <div class="payout-period-calendar__header__year-selection__prev" @click="decrementYear">
                    <GrayArrowLeftIcon />
                </div>
                <p class="payout-period-calendar__header__year-selection__year">{{ displayedYear }}</p>
                <div class="payout-period-calendar__header__year-selection__next" @click="incrementYear">
                    <GrayArrowLeftIcon />
                </div>
            </div>
        </div>
        <div class="payout-period-calendar__months-area">
            <div
                v-for="item in currentDisplayedMonths"
                :key="item.name"
                class="month-item"
                :class="{ selected: item.selected, disabled: !item.active }"
                @click="checkMonth(item)"
            >
                <p class="month-item__label">{{ item.name }}</p>
            </div>
        </div>
        <div class="payout-period-calendar__footer-area">
            <p class="payout-period-calendar__footer-area__period">{{ period }}</p>
            <p class="payout-period-calendar__footer-area__ok-button" @click="submit">OK</p>
        </div>
    </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';

import {
    MonthButton,
    monthNames,
    StoredMonthsByYear,
} from '@/app/types/payout';
import { useAppStore } from '@/app/store/modules/appStore';
import { usePayoutStore } from '@/app/store/modules/payoutStore';

import GrayArrowLeftIcon from '@/../static/images/payments/GrayArrowLeft.svg';

const appStore = useAppStore();
const payoutStore = usePayoutStore();

const now = ref<Date>(new Date());
const currentDisplayedMonths = ref<MonthButton[]>([]);
const displayedYear = ref<number>(now.value.getUTCFullYear());
const period = ref<string>('');
const displayedMonths = ref<StoredMonthsByYear>({});
const selectedMonth = ref<MonthButton | null>(null);

async function submit(): Promise<void> {
    if (selectedMonth.value) {
        const month = selectedMonth.value.index < 9 ? '0' + (selectedMonth.value.index + 1) : (selectedMonth.value.index + 1);
        payoutStore.setPayoutHistoryPeriod(`${selectedMonth.value.year}-${month}`);

        try {
            await payoutStore.fetchPayoutHistory();
        } catch (error) {
            console.error(error);
        }
    }

    close();
}

function updatePeriod(): void {
    if (!selectedMonth.value) {
        period.value = '';

        return;
    }

    period.value = `${monthNames[selectedMonth.value.index]}, ${selectedMonth.value.year}`;
}

function checkMonth(month: MonthButton): void {
    if (!month.active || month.selected) {
        return;
    }

    if (selectedMonth.value) {
        selectedMonth.value.selected = false;
    }

    selectedMonth.value = month;
    month.selected = true;
    updatePeriod();
}

function incrementYear(): void {
    const isCurrentYear = displayedYear.value === now.value.getUTCFullYear();
    if (isCurrentYear) return;

    displayedYear.value += 1;
    populateMonths(displayedYear.value);
    currentDisplayedMonths.value = displayedMonths.value[displayedYear.value];
}

function decrementYear(): void {
    const availableYears: number[] = payoutStore.state.payoutHistoryAvailablePeriods.map(payoutPeriod => payoutPeriod.year);
    const minYear: number = Math.min(...availableYears);

    if (displayedYear.value === minYear) return;

    displayedYear.value -= 1;
    populateMonths(displayedYear.value);
    currentDisplayedMonths.value = displayedMonths.value[displayedYear.value];
}

function populateMonths(year: number): void {
    if (displayedMonths.value[year]) {
        currentDisplayedMonths.value = displayedMonths.value[year];

        return;
    }

    const months: MonthButton[] = [];
    const availablePeriods: string[] = payoutStore.state.payoutHistoryAvailablePeriods.map(payoutPeriod => payoutPeriod.period);

    // Creates months entities and adds them to list.
    for (let i = 0; i < 12; i++) {
        const period = `${year}-${i < 9 ? '0' + (i + 1) : (i + 1)}`;
        const isMonthActive: boolean = availablePeriods.includes(period);

        months.push(new MonthButton(year, i, isMonthActive, false));
    }

    displayedMonths.value[year] = months;
}

function close(): void {
    setTimeout(() => appStore.togglePayoutHistoryCalendar(false), 0);
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
        z-index: 110;

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

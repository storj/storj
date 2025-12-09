// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-click-outside="close" class="payout-period-calendar">
        <div class="payout-period-calendar__header">
            <div class="payout-period-calendar__header__year-selection">
                <div class="payout-period-calendar__header__year-selection__prev" @click.stop="decrementYear">
                    <GrayArrowLeftIcon />
                </div>
                <p class="payout-period-calendar__header__year-selection__year">{{ displayedYear }}</p>
                <div class="payout-period-calendar__header__year-selection__next" @click.stop="incrementYear">
                    <GrayArrowLeftIcon />
                </div>
            </div>
            <p class="payout-period-calendar__header__all-time" @click="selectAllTime">All time</p>
        </div>
        <div class="payout-period-calendar__months-area">
            <div
                v-for="item in currentDisplayedMonths"
                :key="item.name"
                class="month-item"
                :class="{ selected: item.selected, disabled: !item.active }"
                @click.stop="checkMonth(item)"
            >
                <p class="month-item__label">{{ item.name }}</p>
            </div>
        </div>
        <div class="payout-period-calendar__footer-area">
            <p class="payout-period-calendar__footer-area__period">{{ period }}</p>
            <v-button label="Done" :is-disabled="!period" width="73px" height="40px" :on-press="submit" />
        </div>
    </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';

import { UnauthorizedError } from '@/api';
import { monthNames } from '@/app/types/date';
import { MonthButton, StoredMonthsByYear } from '@/app/types/payouts';
import { usePayoutsStore } from '@/app/store/payoutsStore';

import VButton from '@/app/components/common/VButton.vue';

import GrayArrowLeftIcon from '@/../static/images/icons/GrayArrowLeft.svg';

const emit = defineEmits<{
    (e: 'onClose'): void;
}>();

const payoutsStore = usePayoutsStore();

const now = new Date();

const currentDisplayedMonths = ref<MonthButton[]>([]);
const displayedYear = ref<number>(now.getUTCFullYear());
const period = ref<string>('');
const displayedMonths = ref<StoredMonthsByYear>({});
const selectedMonth = ref<MonthButton | null>(null);

async function submit(): Promise<void> {
    let periodValue: string | null = null;

    if (selectedMonth.value) {
        const month = selectedMonth.value.index < 9 ? `0${selectedMonth.value.index + 1}` : selectedMonth.value.index + 1;

        periodValue = `${selectedMonth.value.year}-${month}`;
    }

    payoutsStore.setPayoutPeriod(periodValue);

    try {
        await payoutsStore.summary();
    } catch (error) {
        if (error instanceof UnauthorizedError) {
            // TODO: redirect to login screen.
        }

        // TODO: notify error
    }

    close();
}

function updatePeriod(): void {
    if (!selectedMonth.value) {
        period.value = 'All time';

        return;
    }

    period.value = `${monthNames[selectedMonth.value.index]}, ${selectedMonth.value.year}`;
}

function checkMonth(month: MonthButton): void {
    if (!month.active || month.selected) return;

    if (selectedMonth.value) selectedMonth.value.selected = false;

    selectedMonth.value = month;
    month.selected = true;
    updatePeriod();
}

function selectAllTime(): void {
    if (selectedMonth.value) selectedMonth.value.selected = false;

    selectedMonth.value = null;
    updatePeriod();

    submit();
}

function incrementYear(): void {
    const isCurrentYear = displayedYear.value === now.getUTCFullYear();
    if (isCurrentYear) return;

    displayedYear.value += 1;
    populateMonths(displayedYear.value);
    currentDisplayedMonths.value = displayedMonths.value[displayedYear.value];
}

function decrementYear(): void {
    // TODO: remove hardcoded value
    const minYear = 2000;

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

    // Creates months entities and adds them to list.
    for (let i = 0; i < 12; i++) {
        months.push(new MonthButton(year, i, true, false));
    }

    displayedMonths.value[year] = months;
}

function close(): void {
    emit('onClose');
}

onMounted(() => {
    populateMonths(displayedYear.value);
    currentDisplayedMonths.value = displayedMonths.value[displayedYear.value];
});
</script>

<style scoped lang="scss">
    .payout-period-calendar {
        box-sizing: border-box;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: space-between;
        width: 333px;
        height: 340px;
        background: var(--v-background-base);
        box-shadow: 0 10px 25px rgb(175 183 193 / 10%);
        border-radius: var(--br-block);
        border: 1px solid var(--v-border-base);
        padding: 30px;
        font-family: 'font_regular', sans-serif;
        cursor: default;
        z-index: 1001;

        &__header {
            display: flex;
            flex-direction: row;
            justify-content: space-between;
            align-items: center;
            width: 100%;

            &__year-selection {
                display: flex;
                flex-direction: row;
                align-items: center;
                justify-content: flex-start;

                &__prev,
                &__next {
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    color: var(--v-header-base);
                    cursor: pointer;
                    height: 30px;
                    width: 20px;
                }

                &__prev {
                    margin-right: 20px;
                }

                &__year {
                    font-family: 'font_bold', sans-serif;
                    font-size: 24px;
                    line-height: 28px;
                    color: var(--v-header-base);
                }

                &__next {
                    transform: rotate(180deg);
                    margin-left: 20px;
                }
            }

            &__all-time {
                font-family: 'font_regular', sans-serif;
                font-size: 16px;
                color: var(--v-primary-base);
                cursor: pointer;
            }
        }

        &__months-area {
            display: grid;
            grid-template-columns: 93px 93px 93px;
            gap: 1px;
            background: var(--v-border-base);
            overflow: hidden;
            border-radius: var(--br-table);
            border: 1px solid var(--v-border-base);
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
                font-family: 'font_semiBold', sans-serif;
                font-size: 16px;
                color: var(--v-header-base);
                max-width: 50%;
            }

            &__ok-button {
                font-family: 'font_bold', sans-serif;
                font-size: 16px;
                line-height: 23px;
                color: var(--v-primary-base);
                cursor: pointer;
            }
        }
    }

    .month-item {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 100%;
        height: 44px;
        background: var(--v-background-base);
        border: var(--v-border-base);
        cursor: pointer;

        &__label {
            font-size: 16px;
            line-height: 18px;
            color: var(--v-text-base);
        }
    }

    .disabled {
        background: var(--v-disabled-base);
        cursor: default;

        .month-item__label {
            color: var(--c-gray) !important;
        }
    }

    .selected {
        background: var(--c-primary);

        .month-item__label {
            color: white !important;
        }
    }

    .arrow-icon :deep(path) {
        fill: var(--c-gray);
    }
</style>

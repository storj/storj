// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="period-container" @click.stop="openPeriodDropdown">
        <p class="period-container__label long-text">{{ period }}</p>
        <BlackArrowHide v-if="isCalendarShown" />
        <BlackArrowExpand v-else />
        <PayoutHistoryPeriodCalendar
            v-if="isCalendarShown"
            v-click-outside="closePeriodDropdown"
            class="period-container__calendar"
        />
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { monthNames } from '@/app/types/payout';
import { useAppStore } from '@/app/store/modules/appStore';
import { usePayoutStore } from '@/app/store/modules/payoutStore';

import PayoutHistoryPeriodCalendar from '@/app/components/payments/PayoutHistoryPeriodCalendar.vue';

import BlackArrowExpand from '@/../static/images/BlackArrowExpand.svg';
import BlackArrowHide from '@/../static/images/BlackArrowHide.svg';

const appStore = useAppStore();
const payoutStore = usePayoutStore();

const period = computed<string>(() => {
    if (!payoutStore.state.payoutHistoryPeriod) {
        return '';
    }

    const splittedPeriod = payoutStore.state.payoutHistoryPeriod.split('-');

    return `${monthNames[(parseInt(splittedPeriod[1]) - 1)]}, ${splittedPeriod[0]}`;
});

const isCalendarShown = computed<boolean>(() => {
    return appStore.state.isPayoutHistoryCalendarShown;
});

function openPeriodDropdown(): void {
    appStore.togglePayoutHistoryCalendar(true);
}

function closePeriodDropdown(): void {
    appStore.togglePayoutHistoryCalendar(false);
}
</script>

<style scoped lang="scss">
    .period-container {
        position: relative;
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: center;
        background-color: transparent;
        cursor: pointer;

        &__label {
            margin-right: 8px;
            font-family: 'font_regular', sans-serif;
            font-weight: 500;
            font-size: 16px;
            color: var(--regular-text-color);
        }

        &__calendar {
            position: absolute;
            top: 30px;
            right: 0;
        }
    }

    .active {

        .period-container__label {
            color: var(--navigation-link-color);
        }
    }

    .arrow :deep(path) {
        fill: var(--period-selection-arrow-color);
    }
</style>

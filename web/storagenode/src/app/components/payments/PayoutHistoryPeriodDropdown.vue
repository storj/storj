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

import { APPSTATE_ACTIONS } from '@/app/store/modules/appState';
import { monthNames } from '@/app/types/payout';
import { useStore } from '@/app/utils/composables';

import PayoutHistoryPeriodCalendar from '@/app/components/payments/PayoutHistoryPeriodCalendar.vue';

import BlackArrowExpand from '@/../static/images/BlackArrowExpand.svg';
import BlackArrowHide from '@/../static/images/BlackArrowHide.svg';

const store = useStore();

const period = computed<string>(() => {
    if (!store.state.payoutModule.payoutHistoryPeriod) {
        return '';
    }

    const splittedPeriod = store.state.payoutModule.payoutHistoryPeriod.split('-');

    return `${monthNames[(splittedPeriod[1] - 1)]}, ${splittedPeriod[0]}`;
});

const isCalendarShown = computed<boolean>(() => {
    return store.state.appStateModule.isPayoutHistoryCalendarShown;
});

function openPeriodDropdown(): void {
    store.dispatch(APPSTATE_ACTIONS.TOGGLE_PAYOUT_HISTORY_CALENDAR, true);
}

function closePeriodDropdown(): void {
    store.dispatch(APPSTATE_ACTIONS.TOGGLE_PAYOUT_HISTORY_CALENDAR, false);
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

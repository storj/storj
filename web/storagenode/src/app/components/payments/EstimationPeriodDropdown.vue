// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="period-container" @click.stop="openPeriodDropdown">
        <p class="period-container__label">{{ currentPeriod }}</p>
        <BlackArrowHide v-if="isCalendarShown" />
        <BlackArrowExpand v-else />
        <PayoutPeriodCalendar
            class="period-container__calendar"
            v-click-outside="closePeriodDropdown"
            v-if="isCalendarShown"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import PayoutPeriodCalendar from '@/app/components/payments/PayoutPeriodCalendar.vue';

import BlackArrowExpand from '@/../static/images/BlackArrowExpand.svg';
import BlackArrowHide from '@/../static/images/BlackArrowHide.svg';

import { APPSTATE_ACTIONS } from '@/app/store/modules/appState';
import { PayoutPeriod } from '@/app/types/payout';

/**
 * Holds all months names.
 */
const monthNames = [
    'January', 'February', 'March', 'April',
    'May', 'June', 'July',	'August',
    'September', 'October', 'November',	'December',
];

@Component({
    components: {
        PayoutPeriodCalendar,
        BlackArrowExpand,
        BlackArrowHide,
    },
})
export default class EstimationPeriodDropdown extends Vue {
    /**
     * Returns formatted selected payout period.
     */
    public get currentPeriod(): string {
        const start: PayoutPeriod = this.$store.state.payoutModule.periodRange.start;
        const end: PayoutPeriod = this.$store.state.payoutModule.periodRange.end;

        return start && start.period !== end.period ?
            `${monthNames[start.month]}, ${start.year} - ${monthNames[end.month]}, ${end.year}`
            : `${monthNames[end.month]}, ${end.year}`;
    }

    /**
     * Indicates if period selection calendar should appear.
     */
    public get isCalendarShown(): boolean {
        return this.$store.state.appStateModule.isPayoutCalendarShown;
    }

    /**
     * Opens payout period selection dropdown.
     */
    public openPeriodDropdown(): void {
        this.$store.dispatch(APPSTATE_ACTIONS.TOGGLE_PAYOUT_CALENDAR, true);
    }

    /**
     * Closes payout period selection dropdown.
     */
    public closePeriodDropdown(): void {
        this.$store.dispatch(APPSTATE_ACTIONS.TOGGLE_PAYOUT_CALENDAR, false);
    }
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
            color: var(--month-label-color);
        }

        &__calendar {
            position: absolute;
            top: 30px;
            right: 0;
        }
    }

    .arrow {

        path {
            fill: var(--period-selection-arrow-color);
        }
    }
</style>

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

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import PayoutHistoryPeriodCalendar from '@/app/components/payments/PayoutHistoryPeriodCalendar.vue';

import BlackArrowExpand from '@/../static/images/BlackArrowExpand.svg';
import BlackArrowHide from '@/../static/images/BlackArrowHide.svg';

import { APPSTATE_ACTIONS } from '@/app/store/modules/appState';
import { monthNames } from '@/app/types/payout';

// @vue/component
@Component({
    components: {
        PayoutHistoryPeriodCalendar,
        BlackArrowExpand,
        BlackArrowHide,
    },
})
export default class PayoutHistoryPeriodDropdown extends Vue {
    /**
     * String presentation of selected payout history period.
     */
    public get period(): string {
        if (!this.$store.state.payoutModule.payoutHistoryPeriod) {
            return '';
        }

        const splittedPeriod = this.$store.state.payoutModule.payoutHistoryPeriod.split('-');

        return `${monthNames[(splittedPeriod[1] - 1)]}, ${splittedPeriod[0]}`;
    }

    /**
     * Indicates if period selection calendar should appear.
     */
    public get isCalendarShown(): boolean {
        return this.$store.state.appStateModule.isPayoutHistoryCalendarShown;
    }

    /**
     * Opens payout period selection dropdown.
     */
    public openPeriodDropdown(): void {
        this.$store.dispatch(APPSTATE_ACTIONS.TOGGLE_PAYOUT_HISTORY_CALENDAR, true);
    }

    /**
     * Closes payout period selection dropdown.
     */
    public closePeriodDropdown(): void {
        this.$store.dispatch(APPSTATE_ACTIONS.TOGGLE_PAYOUT_HISTORY_CALENDAR, false);
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

    .arrow ::v-deep path {
        fill: var(--period-selection-arrow-color);
    }
</style>

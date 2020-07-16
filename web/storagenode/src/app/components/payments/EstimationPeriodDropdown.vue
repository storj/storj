// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="period-container" :class="{ disabled: isCalendarDisabled }" @click.stop="openPeriodDropdown">
        <p class="period-container__label long-text">Custom Date Range</p>
        <p class="period-container__label short-text">Custom Range</p>
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

@Component({
    components: {
        PayoutPeriodCalendar,
        BlackArrowExpand,
        BlackArrowHide,
    },
})
export default class EstimationPeriodDropdown extends Vue {
    /**
     * Indicates if period selection calendar should appear.
     */
    public get isCalendarShown(): boolean {
        return this.$store.state.appStateModule.isPayoutCalendarShown;
    }

    /**
     * Indicates if period selection calendar should be disabled.
     */
    public get isCalendarDisabled(): boolean {
        const nodeStartedAt = this.$store.state.node.selectedSatellite.joinDate;
        const now = new Date();

        return nodeStartedAt.getUTCMonth() === now.getUTCMonth() && nodeStartedAt.getUTCFullYear() === now.getUTCFullYear();
    }

    /**
     * Opens payout period selection dropdown.
     */
    public openPeriodDropdown(): void {
        if (this.isCalendarDisabled) {
            return;
        }

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

    .arrow {

        path {
            fill: var(--period-selection-arrow-color);
        }
    }

    .short-text {
        display: none;
    }

    .disabled {

        .period-container {

            &__label {
                color: #909bad;
            }
        }

        .arrow {

            path {
                fill: #909bad !important;
            }
        }
    }

    @media screen and (max-width: 505px) {

        .period-container__label {
            margin-right: 4px;
        }

        .short-text {
            display: inline-block;
            font-size: 14px;
        }

        .long-text {
            display: none;
        }
    }
</style>

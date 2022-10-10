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

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import GrayArrowLeftIcon from '@/../static/images/payments/GrayArrowLeft.svg';

import { APPSTATE_ACTIONS } from '@/app/store/modules/appState';
import { PAYOUT_ACTIONS } from '@/app/store/modules/payout';
import {
    MonthButton,
    monthNames,
    StoredMonthsByYear,
} from '@/app/types/payout';

// @vue/component
@Component({
    components: {
        GrayArrowLeftIcon,
    },
})
export default class PayoutHistoryPeriodCalendar extends Vue {
    private now: Date = new Date();
    /**
     * Contains current months list depends on active and selected month state.
     */
    public currentDisplayedMonths: MonthButton[] = [];
    public displayedYear: number = this.now.getUTCFullYear();
    public period = '';

    private displayedMonths: StoredMonthsByYear = {};
    private selectedMonth: MonthButton | null;

    /**
     * Lifecycle hook after initial render.
     * Sets up current calendar state.
     */
    public mounted(): void {
        this.populateMonths(this.displayedYear);
        this.currentDisplayedMonths = this.displayedMonths[this.displayedYear];
    }

    public async submit(): Promise<void> {
        if (this.selectedMonth) {
            const month = this.selectedMonth.index < 9 ? '0' + (this.selectedMonth.index + 1) : (this.selectedMonth.index + 1);
            await this.$store.dispatch(PAYOUT_ACTIONS.SET_PAYOUT_HISTORY_PERIOD,
                `${this.selectedMonth.year}-${month}`,
            );

            try {
                await this.$store.dispatch(PAYOUT_ACTIONS.GET_PAYOUT_HISTORY);
            } catch (error) {
                console.error(error);
            }
        }

        this.close();
    }

    /**
     * Updates selected period label.
     */
    public updatePeriod(): void {
        if (!this.selectedMonth) {
            this.period = '';

            return;
        }

        this.period = `${monthNames[this.selectedMonth.index]}, ${this.selectedMonth.year}`;
    }

    /**
     * Updates first selected month on click.
     */
    public checkMonth(month: MonthButton): void {
        if (!month.active || month.selected) {
            return;
        }

        if (this.selectedMonth) {
            this.selectedMonth.selected = false;
        }

        this.selectedMonth = month;
        month.selected = true;
        this.updatePeriod();
    }

    /**
     * Increments year and updates current months set.
     */
    public incrementYear(): void {
        const isCurrentYear = this.displayedYear === this.now.getUTCFullYear();

        if (isCurrentYear) return;

        this.displayedYear += 1;
        this.populateMonths(this.displayedYear);
        this.currentDisplayedMonths = this.displayedMonths[this.displayedYear];
    }

    /**
     * Decrement year and updates current months set.
     */
    public decrementYear(): void {
        const availableYears: number[] = this.$store.state.payoutModule.payoutHistoryAvailablePeriods.map(payoutPeriod => payoutPeriod.year);
        const minYear: number = Math.min(...availableYears);

        if (this.displayedYear === minYear) return;

        this.displayedYear -= 1;
        this.populateMonths(this.displayedYear);
        this.currentDisplayedMonths = this.displayedMonths[this.displayedYear];
    }

    /**
     * Sets months set in displayedMonths with year as key.
     */
    private populateMonths(year: number): void {
        if (this.displayedMonths[year]) {
            this.currentDisplayedMonths = this.displayedMonths[year];

            return;
        }

        const months: MonthButton[] = [];
        const availablePeriods: string[] = this.$store.state.payoutModule.payoutHistoryAvailablePeriods.map(payoutPeriod => payoutPeriod.period);

        // Creates months entities and adds them to list.
        for (let i = 0; i < 12; i++) {
            const period = `${year}-${i < 9 ? '0' + (i + 1) : (i + 1)}`;
            const isMonthActive: boolean = availablePeriods.includes(period);

            months.push(new MonthButton(year, i, isMonthActive, false));
        }

        this.displayedMonths[year] = months;
    }

    /**
     * Closes calendar.
     */
    private close(): void {
        setTimeout(() => this.$store.dispatch(APPSTATE_ACTIONS.TOGGLE_PAYOUT_HISTORY_CALENDAR, false), 0);
    }
}
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
            grid-gap: 8px;
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

    .arrow-icon ::v-deep path {
        fill: var(--year-selection-arrow-color);
    }
</style>

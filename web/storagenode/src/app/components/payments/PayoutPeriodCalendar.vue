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

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { APPSTATE_ACTIONS } from '@/app/store/modules/appState';
import { PAYOUT_ACTIONS } from '@/app/store/modules/payout';
import {
    MonthButton,
    monthNames,
    PayoutInfoRange,
    StoredMonthsByYear,
} from '@/app/types/payout';
import { PayoutPeriod } from '@/storagenode/payouts/payouts';

import GrayArrowLeftIcon from '@/../static/images/payments/GrayArrowLeft.svg';

// @vue/component
@Component({
    components: {
        GrayArrowLeftIcon,
    },
})
export default class PayoutPeriodCalendar extends Vue {
    private now: Date = new Date();
    /**
     * Contains current months list depends on active and selected month state.
     */
    public currentDisplayedMonths: MonthButton[] = [];
    public displayedYear: number = this.now.getUTCFullYear();
    public period = '';

    private displayedMonths: StoredMonthsByYear = {};
    private firstSelectedMonth: MonthButton | null = null;
    private secondSelectedMonth: MonthButton | null = null;

    /**
     * Lifecycle hook after initial render.
     * Sets up current calendar state.
     */
    public mounted(): void {
        this.populateMonths(this.displayedYear);
        this.currentDisplayedMonths = this.displayedMonths[this.displayedYear];
    }

    public async submit(): Promise<void> {
        if (!this.firstSelectedMonth) {
            this.close();

            return;
        }

        this.secondSelectedMonth ? await this.$store.dispatch(
            PAYOUT_ACTIONS.SET_PERIODS_RANGE, new PayoutInfoRange(
                new PayoutPeriod(this.firstSelectedMonth.year, this.firstSelectedMonth.index),
                new PayoutPeriod(this.secondSelectedMonth.year, this.secondSelectedMonth.index),
            ),
        ) : await this.$store.dispatch(
            PAYOUT_ACTIONS.SET_PERIODS_RANGE, new PayoutInfoRange(
                null,
                new PayoutPeriod(this.firstSelectedMonth.year, this.firstSelectedMonth.index),
            ),
        );

        try {
            await this.$store.dispatch(PAYOUT_ACTIONS.GET_PAYOUT_INFO, this.$store.state.node.selectedSatellite.id);
            await this.$store.dispatch(APPSTATE_ACTIONS.SET_NO_PAYOUT_DATA, false);
        } catch (error) {
            const lastMonthDate = new Date();
            lastMonthDate.setMonth(lastMonthDate.getUTCMonth() - 1);

            const selectedPeriod: PayoutInfoRange = this.$store.state.payoutModule.periodRange;
            const lastMonthPayoutPeriod = new PayoutPeriod(lastMonthDate.getUTCFullYear(), lastMonthDate.getUTCMonth());
            const isLastPeriodSelected: boolean = !selectedPeriod.start && selectedPeriod.end.period === lastMonthPayoutPeriod.period;

            if (!isLastPeriodSelected) {
                await this.$store.dispatch(APPSTATE_ACTIONS.SET_NO_PAYOUT_DATA, true);
                console.error(error);
            }
        }

        this.close();
    }

    /**
     * Updates selected period label.
     */
    public updatePeriod(): void {
        if (!this.firstSelectedMonth) {
            this.period = '';

            return;
        }

        this.period = this.secondSelectedMonth ?
            `${this.firstSelectedMonth.name}, ${this.firstSelectedMonth.year} - ${this.secondSelectedMonth.name}, ${this.secondSelectedMonth.year}`
            : `${monthNames[this.firstSelectedMonth.index]}, ${this.firstSelectedMonth.year}`;
    }

    /**
     * Selects period between node start and now.
     */
    public selectAllTime(): void {
        const nodeStartedAt = this.$store.state.node.selectedSatellite.joinDate;

        if (nodeStartedAt.getUTCMonth() === this.now.getUTCMonth() && nodeStartedAt.getUTCFullYear() === this.now.getUTCFullYear()) {
            return;
        }

        this.firstSelectedMonth = new MonthButton(nodeStartedAt.getUTCFullYear(), nodeStartedAt.getUTCMonth());
        this.secondSelectedMonth = this.now.getUTCMonth() === 0 ?
            new MonthButton(this.now.getUTCFullYear() - 1, 11)
            : new MonthButton(this.now.getUTCFullYear(), this.now.getUTCMonth() - 1);

        if (
            this.firstSelectedMonth.year === this.secondSelectedMonth.year
            && this.firstSelectedMonth.index === this.secondSelectedMonth.index
        ) {
            this.secondSelectedMonth = null;
            this.checkMonth(this.firstSelectedMonth);
        }

        this.updateMonthsSelection(true);
        this.updatePeriod();
    }

    /**
     * Updates first and second selected month on click.
     */
    public checkMonth(month: MonthButton): void {
        if (this.firstSelectedMonth && this.secondSelectedMonth) {
            this.updateMonthsSelection(false);
            this.firstSelectedMonth = this.secondSelectedMonth = null;

            if (month.active) {
                this.firstSelectedMonth = month;
                month.selected = true;
            }

            this.updatePeriod();

            return;
        }

        if (!month.active) return;

        if (!this.firstSelectedMonth) {
            this.firstSelectedMonth = month;
            month.selected = true;
            this.updatePeriod();

            return;
        }

        if (this.firstSelectedMonth === month) {
            this.firstSelectedMonth = null;
            month.selected = false;
            this.updatePeriod();

            return;
        }

        this.secondSelectedMonth = month;
        if ((this.secondSelectedMonth && this.firstSelectedMonth) && new Date(this.secondSelectedMonth.year, this.secondSelectedMonth.index) < new Date(this.firstSelectedMonth.year, this.firstSelectedMonth.index)) {
            [this.secondSelectedMonth, this.firstSelectedMonth] = [this.firstSelectedMonth, this.secondSelectedMonth];
        }

        this.updatePeriod();
        this.updateMonthsSelection(true);
    }

    /**
     * Increments year and updates current months set.
     */
    public incrementYear(): void {
        if (this.displayedYear === this.now.getUTCFullYear()) return;

        this.displayedYear += 1;
        this.populateMonths(this.displayedYear);
        this.currentDisplayedMonths = this.displayedMonths[this.displayedYear];
    }

    /**
     * Decrement year and updates current months set.
     */
    public decrementYear(): void {
        if (this.displayedYear === this.$store.state.node.selectedSatellite.joinDate.getUTCFullYear()) return;

        this.displayedYear -= 1;
        this.populateMonths(this.displayedYear);
        this.currentDisplayedMonths = this.displayedMonths[this.displayedYear];
    }

    /**
     * Marks all months between first and second selected as selected/unselected.
     */
    private updateMonthsSelection(value: boolean): void {
        if (!this.firstSelectedMonth) return;

        if (!this.secondSelectedMonth) {
            const selectedMonth = this.displayedMonths[this.firstSelectedMonth.year].find(month => {
                if (this.firstSelectedMonth) {
                    return month.index === this.firstSelectedMonth.index;
                }
            });

            if (selectedMonth) {
                selectedMonth.selected = value;
            }

            return;
        }

        for (let i = this.firstSelectedMonth.year; i <= this.secondSelectedMonth.year; i++) {
            if (!this.displayedMonths[i]) {
                this.populateMonths(i);
            }

            this.displayedMonths[i].forEach(month => {
                const date = new Date(month.year, month.index);

                if (
                    (this.secondSelectedMonth && this.firstSelectedMonth)
                    && new Date(this.firstSelectedMonth.year, this.firstSelectedMonth.index) <= date
                    && date <= new Date(this.secondSelectedMonth.year, this.secondSelectedMonth.index)
                ) {
                    month.selected = value;
                }
            });
        }
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
        const availablePeriods: string[] = this.$store.state.payoutModule.payoutPeriods.map(payoutPeriod => payoutPeriod.period);
        const lastMonthDate = new Date();
        lastMonthDate.setMonth(lastMonthDate.getUTCMonth() - 1);

        // Creates month entities and adds them to list.
        for (let i = 0; i < 12; i++) {
            const period = `${year}-${i < 9 ? '0' + (i + 1) : (i + 1)}`;

            const isLastMonth: boolean = lastMonthDate.getUTCFullYear() === year && lastMonthDate.getUTCMonth() === i;
            const isLastMonthActive: boolean =
                isLastMonth && this.$store.state.node.selectedSatellite.joinDate.getTime() < new Date(
                    this.now.getUTCFullYear(), this.now.getUTCMonth(), 1, 0, 0, 1,
                ).getTime();

            const isMonthActive: boolean = availablePeriods.includes(period);

            months.push(new MonthButton(year, i, isMonthActive || isLastMonthActive, false));
        }

        this.displayedMonths[year] = months;
    }

    /**
     * Closes calendar.
     */
    private close(): void {
        setTimeout(() => this.$store.dispatch(APPSTATE_ACTIONS.TOGGLE_PAYOUT_CALENDAR, false), 0);
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

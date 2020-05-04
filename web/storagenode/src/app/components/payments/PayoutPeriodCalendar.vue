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
            <p class="payout-period-calendar__header__all-time" @click="selectAllTime">All time</p>
        </div>
        <div class="payout-period-calendar__months-area">
            <div
                class="month-item"
                :class="{ selected: item.selected, disabled: !item.active }"
                v-for="item in currentDisplayedMonths"
                :key="item.name"
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
import { PayoutInfoRange, PayoutPeriod } from '@/app/types/payout';

interface StoredMonthsByYear {
    [key: number]: MonthButton[];
}

/**
 * Holds all months names.
 */
const monthNames = [
    'January', 'February', 'March', 'April',
    'May', 'June', 'July',	'August',
    'September', 'October', 'November',	'December',
];

/**
 * Describes month button entity for calendar.
 */
class MonthButton {
    public constructor(
        public year: number = 0,
        public index: number = 0,
        public active: boolean = false,
        public selected: boolean = false,
    ) {}

    /**
     * Returns month label depends on index.
     */
    public get name(): string {
        return monthNames[this.index].slice(0, 3);
    }
}

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
    public period: string = '';

    private displayedMonths: StoredMonthsByYear = {};
    private firstSelectedMonth: MonthButton | null;
    private secondSelectedMonth: MonthButton | null;

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

        // TODO: remove checks when buttons will be separated
        if (!this.secondSelectedMonth) {
            const now = new Date();
            if (this.firstSelectedMonth.year === now.getUTCFullYear() && this.firstSelectedMonth.index === now.getUTCMonth()) {
                await this.$store.dispatch(APPSTATE_ACTIONS.SET_NO_PAYOUT_DATA, false);
                await this.$store.dispatch(
                    PAYOUT_ACTIONS.SET_PERIODS_RANGE, new PayoutInfoRange(
                        null,
                        new PayoutPeriod(this.firstSelectedMonth.year, this.firstSelectedMonth.index),
                    ),
                );

                this.close();

                return;
            }
        }
        if (this.secondSelectedMonth && this.secondSelectedMonth.year === this.firstSelectedMonth.year && this.secondSelectedMonth.index === this.firstSelectedMonth.index) {
            await this.$store.dispatch(APPSTATE_ACTIONS.SET_NO_PAYOUT_DATA, false);
            await this.$store.dispatch(
                PAYOUT_ACTIONS.SET_PERIODS_RANGE, new PayoutInfoRange(
                    null,
                    new PayoutPeriod(this.firstSelectedMonth.year, this.firstSelectedMonth.index),
                ),
            );

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
            await this.$store.dispatch(PAYOUT_ACTIONS.GET_HELD_INFO, this.$store.state.node.selectedSatellite.id);
            await this.$store.dispatch(APPSTATE_ACTIONS.SET_NO_PAYOUT_DATA, false);
        } catch (error) {
            await this.$store.dispatch(APPSTATE_ACTIONS.SET_NO_PAYOUT_DATA, true);
            console.error(error.message);
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

        this.firstSelectedMonth = new MonthButton(nodeStartedAt.getUTCFullYear(), nodeStartedAt.getUTCMonth());
        this.secondSelectedMonth = new MonthButton(this.now.getUTCFullYear(), this.now.getUTCMonth());
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
        if (!this.secondSelectedMonth || !this.firstSelectedMonth) return;

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
        const isCurrentYear = year === this.now.getUTCFullYear();
        const nowMonth = this.now.getUTCMonth();
        const nodeStartedAt = this.$store.state.node.selectedSatellite.joinDate;

        for (let i = 0; i < 12; i++) {
            const notBeforeNodeStart =
                nodeStartedAt.getUTCFullYear() < year
                || (nodeStartedAt.getUTCFullYear() === year && nodeStartedAt.getUTCMonth() <= i);
            const inFuture = isCurrentYear && i > nowMonth;

            const isMonthActive = notBeforeNodeStart && !inFuture;
            months.push(new MonthButton(year, i, isMonthActive, false));
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
        box-shadow: 0 10px 25px rgba(175, 183, 193, 0.1);
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

    .arrow-icon {

        path {
            fill: var(--year-selection-arrow-color);
        }
    }
</style>

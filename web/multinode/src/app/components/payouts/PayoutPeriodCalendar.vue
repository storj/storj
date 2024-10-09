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

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { UnauthorizedError } from '@/api';
import { monthNames } from '@/app/types/date';
import { MonthButton, StoredMonthsByYear } from '@/app/types/payouts';

import VButton from '@/app/components/common/VButton.vue';

import GrayArrowLeftIcon from '@/../static/images/icons/GrayArrowLeft.svg';

// @vue/component
@Component({
    components: {
        VButton,
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

    /**
     * Fetches payout information.
     */
    public async submit(): Promise<void> {
        let period: string | null = null;

        if (this.selectedMonth) {
            const month = this.selectedMonth.index < 9 ? `0${this.selectedMonth.index + 1}` : this.selectedMonth.index + 1;

            period = `${this.selectedMonth.year}-${month}`;
        }

        this.$store.commit('payouts/setPayoutPeriod', period);

        try {
            await this.$store.dispatch('payouts/summary');
        } catch (error) {
            if (error instanceof UnauthorizedError) {
                // TODO: redirect to login screen.
            }

            // TODO: notify error
        }

        this.close();
    }

    /**
     * Updates selected period label.
     */
    public updatePeriod(): void {
        if (!this.selectedMonth) {
            this.period = 'All time';

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
     * selectAllTime resets selected payout period.
     */
    public selectAllTime(): void {
        if (this.selectedMonth) {
            this.selectedMonth.selected = false;
        }

        this.selectedMonth = null;
        this.updatePeriod();

        this.submit();
    }

    /**
     * Increments year and updates current months set.
     */
    public incrementYear(): void {
        const isCurrentYear = this.displayedYear === this.now.getUTCFullYear();

        if (isCurrentYear) { return; }

        this.displayedYear += 1;
        this.populateMonths(this.displayedYear);
        this.currentDisplayedMonths = this.displayedMonths[this.displayedYear];
    }

    /**
     * Decrement year and updates current months set.
     */
    public decrementYear(): void {
        // TODO: remove hardcoded value
        const minYear = 2000;

        if (this.displayedYear === minYear) { return; }

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

        // Creates months entities and adds them to list.
        for (let i = 0; i < 12; i++) {
            months.push(new MonthButton(year, i, true, false));
        }

        this.displayedMonths[year] = months;
    }

    /**
     * Closes calendar.
     */
    private close(): void {
        this.$emit('onClose');
    }
}
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
        background: white;
        box-shadow: 0 10px 25px rgb(175 183 193 / 10%);
        border-radius: var(--br-block);
        border: 1px solid #e1e3e6;
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
                    color: var(--c-title);
                }

                &__next {
                    transform: rotate(180deg);
                    margin-left: 20px;
                }
            }

            &__all-time {
                font-family: 'font_regular', sans-serif;
                font-size: 16px;
                color: var(--c-primary);
                cursor: pointer;
            }
        }

        &__months-area {
            display: grid;
            grid-template-columns: 93px 93px 93px;
            grid-gap: 1px;
            background: var(--c-gray--light);
            overflow: hidden;
            border-radius: var(--br-table);
            border: 1px solid var(--c-gray--light);
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
                color: var(--c-payout-period);
                max-width: 50%;
            }

            &__ok-button {
                font-family: 'font_bold', sans-serif;
                font-size: 16px;
                line-height: 23px;
                color: var(--c-button-common);
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
        background: white;
        cursor: pointer;

        &__label {
            font-size: 16px;
            line-height: 18px;
            color: var(--c-title);
        }
    }

    .disabled {
        background: var(--c-button-disabled);
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

    .arrow-icon ::v-deep path {
        fill: var(--c-gray);
    }
</style>

// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payout-period-calendar">
        <div class="payout-period-calendar__header">
            <div class="payout-period-calendar__header__year-selection">
                <div class="payout-period-calendar__header__year-selection__prev">
                    <GrayArrowLeftIcon />
                </div>
                <p class="payout-period-calendar__header__year-selection__year">2020</p>
                <div class="payout-period-calendar__header__year-selection__next">
                    <GrayArrowLeftIcon />
                </div>
            </div>
            <p class="payout-period-calendar__header__all-time">All time</p>
        </div>
        <div class="payout-period-calendar__months-area">
            <div
                class="month-item"
                :class="{ selected: item.selected, disabled: !item.active }"
                v-for="item in displayedMonths"
                :key="item.name"
            >
                <p class="month-item__label">{{ item.name }}</p>
            </div>
        </div>
        <div class="payout-period-calendar__footer-area">
            <p class="payout-period-calendar__footer-area__period">Jan - Apr, 2019</p>
            <p class="payout-period-calendar__footer-area__ok-button">OK</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import GrayArrowLeftIcon from '@/../static/images/payments/GrayArrowLeft.svg';

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
        public index: number = 0,
        public active: boolean = false,
        public selected: boolean = false,
    ) {}

    /**
     * Returns month label depends on index.
     */
    public get name() {
        return monthNames[this.index].slice(0, 3);
    }
}

@Component({
    components: {
        GrayArrowLeftIcon,
    }
})
export default class PayoutPeriodCalendar extends Vue {
    /**
     * Contains current months list depends on active and selected month state.
     */
    public displayedMonths: MonthButton[] = [];

    public constructor() {
        super();

        for (let i = 0; i < monthNames.length; i++) {
            this.displayedMonths.push(new MonthButton(i, true, false));
        }
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
        background: #fff;
        box-shadow: 0 10px 25px rgba(175, 183, 193, 0.1);
        border-radius: 5px;
        padding: 24px;
        font-family: 'font_regular', sans-serif;

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
                    color: #444c63;
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
                color: #224ca5;
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
                color: #444c63;
            }

            &__ok-button {
                font-family: 'font_bold', sans-serif;
                font-size: 16px;
                line-height: 23px;
                color: #224ca5;
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
        background: #f1f4f9;
        border-radius: 10px;
        cursor: pointer;

        &__label {
            font-size: 12px;
            line-height: 18px;
            color: #667086;
        }
    }

    .disabled {
        background: #e9e9e9;
        cursor: default;

        .month-item__label {
            color: #b1b1b1 !important;
        }
    }

    .selected {
        background: #224ca5;

        .month-item__label {
            color: white !important;
        }
    }
</style>

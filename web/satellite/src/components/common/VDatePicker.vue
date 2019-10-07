// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="cov-vue-date">
        <div class="datepickbox">
            <input type="text" title="input date" class="cov-datepicker" readonly="readonly" :style="option.inputStyle ? option.inputStyle : {}" />
        </div>
        <div class="datepicker-overlay" v-if="isChecking" @click.self="dismiss" :style="{'background' : option.overlayOpacity? 'rgba(0,0,0,'+option.overlayOpacity+')' : 'rgba(0,0,0,0.5)'}">
            <div class="cov-date-body" :style="{'background-color': option.color ? option.color.header : '#3f51b5'}">
                <div class="cov-date-monthly">
                    <div class="cov-date-previous" @click="onPreviousMonthClick">«</div>
                    <div class="cov-date-caption" :style="{'color': option.color ? option.color.headerText : '#fff'}">
                        <span class="year-selection" @click="showYear">{{selectedDateState.year}}</span>
                        <span class="month-selection" @click="showMonth">{{displayedMonth}}</span>
                    </div>
                    <div class="cov-date-next" @click="onNextMonthClick">»</div>
                </div>
                <div class="cov-date-box" v-if="isDaysChoiceShown">
                    <div class="cov-picker-box">
                        <div class="week">
                            <ul>
                                <li v-for="week in daysInWeek" :key="week">{{week}}</li>
                            </ul>
                        </div>
                        <div class="day" v-for="(day, index) in daysToShow" :key="index" @click="checkDay(day)" :class="{'checked':day.checked,'unavailable':day.unavailable,'passive-day': !(day.inMonth), 'today': day.today}" :style="day.checked ? (option.color && option.color.checkedDay ? { background: option.color.checkedDay } : { background: '#2683FF' }) : {}">{{day.value}}</div>
                    </div>
                </div>
                <div class="cov-date-box list-box" v-if="isYearChoiceShown">
                    <div class="cov-picker-box date-list" id="yearList">
                        <div class="date-item year" v-for="yearItem in years" :key="yearItem" @click="setYear(yearItem)">{{yearItem}}</div>
                    </div>
                </div>
                <div class="cov-date-box list-box" v-if="isMonthChoiceShown">
                    <div class="cov-picker-box date-list">
                        <div class="date-item month" v-for="monthItem in monthsNames" :key="monthItem" @click="setMonth(monthItem)">{{monthItem}}</div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>
<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import {
    DateGenerator,
    DateStamp,
    DayAction,
    DayItem,
    DisplayedType,
    Options,
} from '@/utils/datepicker';

@Component
export default class VDatePicker extends Vue {
    @Prop({default: () => new Options()})
    private option: Options;
    @Prop({default: () => false})
    private isSundayFirst: boolean;

    private readonly MAX_DAYS_SELECTED: number = 2;
    public selectedDays: Date[] = [];

    private showType: number = DisplayedType.Day;
    private dateGenerator: DateGenerator = new DateGenerator();

    // daysInWeek contains days names abbreviations
    public readonly daysInWeek: string[] = [];
    public readonly monthsNames: string[] = [];
    // years contains years numbers available to choose
    public readonly years: number[] = [];

    // isChecking indicates when calendar is shown
    public isChecking: boolean = false;
    public displayedMonth: string;
    // daysToShow contains days of selected month with a few extra day from adjacent months
    public daysToShow: DayItem[] = [];

    // Combination of selected year, month and day
    public selectedDateState: DateStamp = new DateStamp(0, 0, 0);

    public constructor() {
        super();

        this.daysInWeek = this.isSundayFirst ? this.option.sundayFirstWeek : this.option.mondayFirstWeek;
        this.monthsNames = this.option.month;
        this.displayedMonth = this.monthsNames[0];
        this.years = this.dateGenerator.populateYears();
    }

    /**
     * computed value that indicates should days view be shown
     */
    public get isDaysChoiceShown(): boolean {
        return this.showType === DisplayedType.Day;
    }

    /**
     * computed value that indicates should month choice view be shown
     */
    public get isMonthChoiceShown(): boolean {
        return this.showType === DisplayedType.Month;
    }

    /**
     * computed value that indicates should year choice view be shown
     */
    public get isYearChoiceShown(): boolean {
        return this.showType === DisplayedType.Year;
    }

    /**
     * onPreviousMonthClick set previous month
     */
    public onPreviousMonthClick(): void {
        this.nextMonth(DayAction.Previous);
    }

    /**
     * onNextMonthClick set next month
     */
    public onNextMonthClick(): void {
        this.nextMonth(DayAction.Next);
    }

    /**
     * checkDay toggles checked property of day object
     *
     * @param day represent day object to check/uncheck
     */
    public checkDay(day: DayItem): void {
        if (day.unavailable || !day.value) {
            return;
        }

        if (!day.inMonth) {
            this.nextMonth(day.action);

            return;
        }

        if (day.checked) {
            day.checked = false;
            this.selectedDays.splice(this.selectedDays.indexOf(day.moment), 1);

            return;
        }

        if (this.selectedDays.length < this.MAX_DAYS_SELECTED) {
            this.selectedDays.push(day.moment);
            day.checked = true;
        }

        if (this.selectedDays.length === this.MAX_DAYS_SELECTED) {
            this.submitSelectedDays();
        }
    }

    /**
     * setYear selects chosen year
     *
     * @param year
     */
    public setYear(year): void {
        this.populateDays(new Date(year, this.selectedDateState.month, this.selectedDateState.day));
    }

    /**
     * setYear selects chosen month
     *
     * @param month
     */
    public setMonth(month: string): void {
        const monthIndex = this.monthsNames.indexOf(month);
        this.populateDays(new Date(this.selectedDateState.year, monthIndex, this.selectedDateState.day));
    }

    /**
     * dismiss closes popup and clears values
     */
    public dismiss(): void {
        if (!this.option.dismissible) {
            return;
        }

        this.selectedDays = [];
        this.isChecking = false;
    }

    /**
     * showCheck used for external popup opening
     */
    public showCheck(): void {
        this.populateDays();
        this.isChecking = true;
    }

    /**
     * showYear used for opening choose year view
     */
    public showYear(): void {
        this.showType = DisplayedType.Year;
    }

    /**
     * showMonth used for opening choose month view
     */
    public showMonth(): void {
        this.showType = DisplayedType.Month;
    }

    /**
     * nextMonth set month depends on day action (next or previous)
     *
     * @param action represents next or previous type of action
     */
    private nextMonth(action: DayAction): void {
        const currentMoment = new Date(this.selectedDateState.year, this.selectedDateState.month, this.selectedDateState.day);
        const currentMonth = currentMoment.getMonth();
        const now = new Date();

        switch (action) {
            case DayAction.Next:
                if (currentMonth === now.getMonth() && currentMoment.getFullYear() === now.getFullYear()) {
                    return;
                }

                currentMoment.setMonth(currentMonth + 1);
                break;
            case DayAction.Previous:
                currentMoment.setMonth(currentMonth - 1);
                break;
        }

        this.populateDays(currentMoment);
    }

    /**
     * submitSelectedDays emits function to receive selected dates externally and then clears state
     */
    private submitSelectedDays(): void {
        this.$emit('change', this.selectedDays);
        this.isChecking = false;
        this.selectedDays = [];
    }

    /**
     * populateDays used for populating date items into calendars depending on selected date
     *
     * @param date represents Date which is used to create current date items to show
     */
    private populateDays(date: Date = new Date()): void {
        this.selectedDateState.fromDate(date);
        this.showType = DisplayedType.Day;
        this.displayedMonth = this.monthsNames[this.selectedDateState.month];
        this.daysToShow = this.dateGenerator.populateDays(this.selectedDateState, this.isSundayFirst);
    }
}
</script>

<style scoped lang="scss">
    .datepicker-overlay {
        position: fixed;
        width: 100%;
        height: 100%;
        z-index: 998;
        top: 0;
        left: 0;
        overflow: hidden;
        -webkit-animation: fadein 0.5s;
        /* Safari, Chrome and Opera > 12.1 */
        -moz-animation: fadein 0.5s;
        /* Firefox < 16 */
        -ms-animation: fadein 0.5s;
        /* Internet Explorer */
        -o-animation: fadein 0.5s;
        /* Opera < 12.1 */
        animation: fadein 0.5s;
    }

    @keyframes fadein {
        from {
            opacity: 0;
        }
        to {
            opacity: 1;
        }
    }

    /* Firefox < 16 */
    @-moz-keyframes fadein {
        from {
            opacity: 0;
        }
        to {
            opacity: 1;
        }
    }

    /* Safari, Chrome and Opera > 12.1 */
    @-webkit-keyframes fadein {
        from {
            opacity: 0;
        }
        to {
            opacity: 1;
        }
    }

    /* Internet Explorer */
    @-ms-keyframes fadein {
        from {
            opacity: 0;
        }
        to {
            opacity: 1;
        }
    }

    /* Opera < 12.1 */
    @-o-keyframes fadein {
        from {
            opacity: 0;
        }
        to {
            opacity: 1;
        }
    }

    .cov-date-body {
        background: #3F51B5;
        overflow: hidden;
        font-size: 16px;
        font-weight: 400;
        position: fixed;
        display: block;
        width: 400px;
        max-width: 100%;
        z-index: 999;
        top: 50%;
        left: 50%;
        -webkit-transform: translate(-50%, -50%);
        -ms-transform: translate(-50%, -50%);
        transform: translate(-50%, -50%);
        box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.2);
        font-family: 'font_medium';
    }

    .cov-picker-box {
        background: #fff;
        display: inline-block;
        padding: 25px;
        box-sizing: border-box !important;
        -moz-box-sizing: border-box !important;
        -webkit-box-sizing: border-box !important;
        -ms-box-sizing: border-box !important;
        width: 400px;
        max-width: 100%;
        height: 280px;
        text-align: start !important;
    }

    .day {
        width: 14.2857143%;
        display: inline-block;
        text-align: center;
        cursor: pointer;
        height: 34px;
        padding: 0;
        line-height: 34px;
        color: #000;
        background: #fff;
        vertical-align: middle;
    }

    .week ul {
        margin: 0 0 8px;
        padding: 0;
        list-style: none;
    }

    .week ul li {
        width: 14.2%;
        display: inline-block;
        text-align: center;
        background: transparent;
        color: #000;
        font-weight: bold;
    }

    .passive-day {
        color: #bbb;
    }

    .checked {
        background: #2683FF;
        color: #FFF !important;
    }

    .unavailable {
        color: #ccc;
        cursor: not-allowed;
    }

    .cov-date-monthly {
        height: 50px;
    }

    .cov-date-monthly > div {
        display: inline-block;
        padding: 0;
        margin: 0;
        vertical-align: middle;
        color: #fff;
        height: 50px;
        float: left;
        text-align: center;
    }

    .cov-date-previous,
    .cov-date-next {
        position: relative;
        width: 20% !important;
        text-indent: -300px;
        overflow: hidden;
        color: #fff;
    }

    .cov-date-caption {
        width: 60%;
        padding: 10px 0 !important;
        box-sizing: border-box;
        font-size: 18px;
        font-family: 'font_medium';
        line-height: 30px;
        display: flex;
        flex-direction: column;
        justify-content: center;
        align-items: center;
    }

    .month-selection,
    .year-selection {
        padding: 0 3px;
    }

    .cov-date-previous:hover,
    .cov-date-next:hover {
        background: rgba(255, 255, 255, 0.1);
    }

    .day:hover {
        background: #EAEAEA;
    }

    .unavailable:hover {
        background: none;
    }

    .cov-date-next::before,
    .cov-date-previous::before {
        width: 20px;
        height: 2px;
        text-align: center;
        position: absolute;
        background: #fff;
        top: 50%;
        margin-top: -7px;
        margin-left: -7px;
        left: 50%;
        line-height: 0;
        content: '';
        -webkit-transform: rotate(45deg);
        -moz-transform: rotate(45deg);
        transform: rotate(45deg);
    }

    .cov-date-next::after,
    .cov-date-previous::after {
        width: 20px;
        height: 2px;
        text-align: center;
        position: absolute;
        background: #fff;
        margin-top: 6px;
        margin-left: -7px;
        top: 50%;
        left: 50%;
        line-height: 0;
        content: '';
        -webkit-transform: rotate(-45deg);
        -moz-transform: rotate(-45deg);
        transform: rotate(-45deg);
    }

    .cov-date-previous::after {
        -webkit-transform: rotate(45deg);
        -moz-transform: rotate(45deg);
        transform: rotate(45deg);
    }

    .cov-date-previous::before {
        -webkit-transform: rotate(-45deg);
        -moz-transform: rotate(-45deg);
        transform: rotate(-45deg);
    }

    .date-item {
        text-align: center;
        font-size: 20px;
        padding: 10px 0;
        cursor: pointer;
    }

    .date-item:hover {
        background: #e0e0e0;
    }

    .date-list {
        overflow: auto;
        vertical-align: top;
        padding: 0;
    }

    .cov-vue-date {
        display: inline-block;
        color: #5D5D5D;
    }

    .watch-box {
        height: 100%;
        overflow: hidden;
    }

    ::-webkit-scrollbar {
        width: 2px;
    }

    ::-webkit-scrollbar-track {
        background: #E3E3E3;
    }

    ::-webkit-scrollbar-thumb {
        background: #C1C1C1;
        border-radius: 2px;
    }

    .cov-date-box {
        font-family: 'font_medium';
    }

    .today {
        background: lightblue;
        color: white;
    }
</style>

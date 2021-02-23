// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Options is a set of options used for VDatePicker.vue.
 */
export class Options {
    public constructor(
        public mondayFirstWeek: string[] = ['Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa', 'Su'],
        public sundayFirstWeek: string[] = ['Su', 'Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa'],
        public month: string[] = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'],
        public color = {
            checked: '#2683FF',
            header: '#2683FF',
            headerText: '#444C63',
        },
        public inputStyle = {
            'visibility': 'hidden',
            'width': '0',
        },
        public overlayOpacity: number = 0.5,
        public dismissible: boolean = true,
    ) {}
}

/**
 * DayItem is used to store information about day cell in calendar.
 */
export class DayItem {
    public constructor(
        public value: number,
        public inMonth: boolean,
        public unavailable: boolean,
        public checked: boolean,
        public moment: Date,
        public action: DayAction = DayAction.Default,
        public today: boolean = false,
    ) {}

    public equals(dateToCompare: Date): boolean {
        const isDayEqual = this.moment.getDate() === dateToCompare.getDate();
        const isMonthEqual = this.moment.getMonth() === dateToCompare.getMonth();
        const isYearEqual = this.moment.getFullYear() === dateToCompare.getFullYear();

        return isDayEqual && isMonthEqual && isYearEqual;
    }
}

/**
 * DayAction is enum represents month change direction on day click.
 */
export enum DayAction {
    Next,
    Previous,
    Default,
}

/**
 * DateStamp is cozy representation of Date for view.
 */
export class DateStamp {
    public constructor(
        public year: number,
        public month: number,
        public day: number,
    ) {}

    public fromDate(date: Date): void {
        this.year = date.getFullYear();
        this.month = date.getMonth();
        this.day = date.getDate();
    }
}

/**
 * DisplayedType is enum represents view type to show in calendar to check.
 */
export enum DisplayedType {
    Day,
    Month,
    Year,
}

/**
 * DateGenerator is utility class used for generating DayItem and year lists for calendar.
 */
export class DateGenerator {
    private current: DateStamp;
    private isSundayFirst: boolean;
    private now = new Date();

    public populateDays(current: DateStamp, isSundayFirst: boolean): DayItem[] {
        this.current = current;
        this.isSundayFirst = isSundayFirst;

        const days: DayItem[] = [];

        this.populateSelectedMonthDays(days);
        this.populatePreviousMonthDays(days);
        this.populateNextMonthDays(days);
        this.markToday(days);

        return days;
    }

    public populateYears(): number[] {
        const year = new Date().getFullYear();
        const years: number[] = [];
        for (let i = year - 99; i <= year; i++) {
            years.unshift(i);
        }

        return years;
    }

    private populateSelectedMonthDays(days: DayItem[]): void {
        const daysInSelectedMonth = new Date(this.current.year, this.current.month + 1, 0).getDate();
        const currentMonth = this.now.getMonth();

        for (let i = 1; i <= daysInSelectedMonth; i++) {
            const moment = new Date(this.current.year, this.current.month, this.current.day, 23, 59);
            moment.setDate(i);

            days.push(
                new DayItem(
                    i,
                    this.current.month !== currentMonth || (this.current.month === currentMonth && i <= this.now.getDate()),
                    false,
                    false,
                    moment,
                ),
            );
        }
    }

    private populatePreviousMonthDays(days: DayItem[]): void {
        const previousMonth = new Date(this.current.year, this.current.month, this.current.day);
        previousMonth.setMonth(previousMonth.getMonth() - 1);

        const firstDate = new Date(this.current.year, this.current.month, this.current.day);
        firstDate.setDate(1);
        let firstDay = firstDate.getDay();

        if (firstDay === 0) firstDay = 7;
        const daysInPreviousMonth = new Date(previousMonth.getFullYear(), previousMonth.getMonth() + 1, 0).getDate();

        for (let i = 0; i < firstDay - (this.isSundayFirst ? 0 : 1); i++) {
            const moment = new Date(this.current.year, this.current.month, this.current.day, 23, 59);
            moment.setDate(1);
            moment.setMonth(moment.getMonth() - 1);
            moment.setDate(new Date(moment.getFullYear(), moment.getMonth() + 1, 0).getDate() - i);

            days.unshift(
                new DayItem(
                    daysInPreviousMonth - i,
                    false,
                    false,
                    false,
                    moment,
                    DayAction.Previous,
                ),
            );
        }
    }

    private populateNextMonthDays(days: DayItem[]): void {
        const passiveDaysAtFinal = 42 - days.length;

        for (let i = 1; i <= passiveDaysAtFinal; i++) {
            const moment = new Date(this.current.year, this.current.month, this.current.day, 23, 59);
            moment.setMonth(moment.getMonth() + 1);
            moment.setDate(i);

            days.push(
                new DayItem(
                    i,
                    false,
                    false,
                    false,
                    moment,
                    DayAction.Next,
                ),
            );
        }
    }

    private markToday(days: DayItem[]): void {
        const daysCount = days.length;

        for (let i = 0; i < daysCount; i++) {
            const day: DayItem = days[i];

            if (day.equals(this.now)) {
                day.today = true;
                break;
            }
        }
    }
}

/**
 * DateFormat is utils class for date formatting to string.
 */
export class DateFormat {

    /**
     * getUSDate transforms date into US date format string.
     * @param date - Date to format
     * @param separator - symbol for joining date string
     * @returns formatted date string
     */
    public static getUTCDate(date: Date, separator: string): string {
        const month = date.getUTCMonth() + 1;
        const day = date.getUTCDate();
        const year = date.getUTCFullYear();

        return [month, day, year].join(separator);
    }
}

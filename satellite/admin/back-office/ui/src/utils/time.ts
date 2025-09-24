// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Time holds methods to operate over timestamps.
 */
export class Time {
    /**
     * formattedDate formats given date into US string.
     * @param date
     * @param options - date formatting options
     */
    public static formattedDate(date: Date | string, options: Intl.DateTimeFormatOptions = { day: 'numeric', month: 'short', year: 'numeric' }): string {
        if (typeof date === 'string') {
            date = new Date(date);
        }
        return date.toLocaleDateString('en-US', options);
    }
}

/**
 * This class simplifies working with duration (nanoseconds) sent from the backend.
 * */
export class Duration {
    static DAY_30 = new Duration(2.592e+15);
    static DAY_60 = new Duration(Duration.DAY_30.nanoseconds * 2);
    static DAY_180 = new Duration(Duration.DAY_30.nanoseconds * 6);
    static YEAR_1 = new Duration(3.1536e+16);

    public constructor(
        public nanoseconds: number,
    ) {}

    /**
     * parsed returns an object of the time components in this.nanoseconds
     * */
    get parsed(): { years: number, days: number, hours: number; seconds: number; minutes: number; milliseconds: number } {
        const milliseconds = Math.floor(this.nanoseconds / 1000000);
        const seconds = Math.floor(milliseconds / 1000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);
        const days = Math.floor(hours / 24);
        const years = Math.floor(days / 365);

        return {
            years,
            days,
            hours: hours % 24,
            minutes: minutes % 60,
            seconds: seconds % 60,
            milliseconds: milliseconds,
        };
    }

    get years(): number {
        return this.parsed.years;
    }

    get days(): number {
        return this.parsed.days;
    }

    get hours(): number {
        return this.parsed.hours;
    }

    get minutes(): number {
        return this.parsed.minutes;
    }

    get seconds(): number {
        return this.parsed.seconds;
    }

    get milliseconds(): number {
        return this.parsed.milliseconds;
    }

    /**
     * shortString represents this duration in the appropriate unit.
     * */
    get shortString(): string {
        let numberPart = this.seconds;
        let unitPart = 'second';
        if (this.years > 0) {
            numberPart = this.years;
            unitPart = 'year';
        } else if (this.days > 0) {
            numberPart = this.days;
            unitPart = 'day';
        } else if (this.hours > 0) {
            numberPart = this.hours;
            unitPart = 'hour';
        } else if (this.minutes > 0) {
            numberPart = this.minutes;
            unitPart = 'minute';
        }

        if (numberPart > 1) {
            unitPart = `${unitPart}s`;
        }
        return `${numberPart} ${unitPart}`;
    }
}

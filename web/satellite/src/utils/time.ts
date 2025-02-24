// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Time holds methods to operate over timestamps.
 */
export class Time {
    /**
     * toUnixTimestamp converts Date to unix timestamp.
     * @param time
     */
    public static toUnixTimestamp(time: Date): number {
        return Math.floor(time.getTime() / 1000);
    }

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

    /**
     * formattedDate formats given date into US string along with GMT offset.
     * This is used to avoid usage of timezone abbreviations (e.g. EET).
     * @param date
     */
    public static formattedDateWithGMTOffset(date: Date): string {
        const formattedDate = date.toLocaleDateString('en-GB', {
            day: 'numeric',
            month: 'short',
            year: 'numeric',
        });

        const offsetMinutes = date.getTimezoneOffset();
        const absoluteOffset = Math.abs(offsetMinutes);
        const offsetHours = Math.floor(absoluteOffset / 60);
        const offsetRemainingMinutes = absoluteOffset % 60;
        const sign = offsetMinutes <= 0 ? '+' : '-';

        let formattedOffset = `GMT${sign}${offsetHours}`;
        if (offsetRemainingMinutes > 0) {
            formattedOffset += `:${offsetRemainingMinutes.toString().padStart(2, '0')}`;
        }

        return `${formattedDate} ${formattedOffset}`;
    }
}

/**
 * This class simplifies working with duration (nanoseconds) sent from the backend.
 * */
export class Duration {
    static ZERO = new Duration(0);
    static MINUTES_15 = new Duration(9e+11);
    static MINUTES_30 = new Duration(1.8e+12);
    static HOUR_1 = new Duration(3.6e+12);
    static DAY_1 = new Duration(8.64e+13);
    static WEEK_1 = new Duration(6.048e+14);
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
    get parsed(): { years: number, days: number, hours: number; seconds: number; minutes: number } {
        const seconds = Math.floor((this.nanoseconds / 1000000) / 1000);
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

    get fullSeconds(): number {
        return Math.floor((this.nanoseconds / 1000000) / 1000);
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

    get remainingFormatted(): string {
        const seconds = Math.floor(this.nanoseconds / 1000000000);
        const remainingHours = Math.floor(seconds / 3600);
        const remainingMinutes = Math.floor((seconds % 3600) / 60);
        const remainingSeconds = Math.floor(seconds % 60);

        let timeString = '';
        if (remainingHours > 0) {
            timeString += `${remainingHours}h `;
        }
        if (remainingMinutes > 0) {
            timeString += `${remainingMinutes}m `;
        }
        timeString += `${remainingSeconds}s`;

        return timeString;
    }
}

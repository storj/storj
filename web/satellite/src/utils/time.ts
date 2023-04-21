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
}

/**
 * This class simplifies working with duration (nanoseconds) sent from the backend.
 * */
export class Duration {

    static MINUTES_15 = new Duration(9e+11);
    static MINUTES_30 = new Duration(1.8e+12);
    static HOUR_1 = new Duration(3.6e+12);
    static DAY_1 = new Duration(8.64e+13);
    static WEEK_1 = new Duration(6.048e+14);
    static DAY_30 = new Duration(2.592e+15);

    public constructor(
        public nanoseconds: number,
    ) {}

    /**
     * parsed returns an object of the time components in this.nanoseconds
     * */
    get parsed(): { days: number, hours: number; seconds: number; minutes: number } {
        const seconds = Math.floor((this.nanoseconds / 1000000) / 1000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);
        const days = Math.floor(hours / 24);

        return {
            days,
            hours: hours % 24,
            minutes: minutes % 60,
            seconds: seconds % 60,
        };
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

    /**
     * shortString represents this duration in the appropriate unit.
     * */
    get shortString(): string {
        let numberPart = this.seconds;
        let unitPart = 'second';
        if (this.days > 0) {
            numberPart = this.days;
            unitPart = 'day';
        } else if (this.hours > 0) {
            numberPart = this.hours;
            unitPart = 'hour';
        } if (this.minutes > 0) {
            numberPart = this.minutes;
            unitPart = 'minute';
        }

        if (numberPart > 1) {
            unitPart = `${unitPart}s`;
        }
        return `${numberPart} ${unitPart}`;
    }

    public isEqualTo(other: Duration): boolean {
        return this.nanoseconds === other.nanoseconds;
    }
}

// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

export class ChartUtils {
    /**
     * Used to display correct number of days on chart's labels.
     *
     * @returns daysDisplayed - array of days converted to a string by using the current locale
     */
    public static daysDisplayedOnChart(start: Date, end: Date): string[] {
        const arr = Array<string>();

        if (start === end) {
            arr.push(`${start.getMonth() + 1}/${start.getDate()}`);

            return arr;
        }

        const dt = start;
        while (dt <= end) {
            arr.push(`${dt.getMonth() + 1}/${dt.getDate()}`);
            dt.setDate(dt.getDate() + 1)
        }

        return arr;
    }
}

// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { DataStamp } from "@/types/projects";

export class ChartUtils {
    /**
     * Adds missing usage for chart data for each day of date range.
     * @param fetchedData - array of data that is spread over missing usage for each day of the date range
     * @param since - instance of since date
     * @param before - instance of before date
     * @returns chartData - array of filled data
     */
    public static populateEmptyUsage(fetchedData: DataStamp[], since: Date, before: Date): DataStamp[] {
        // Create an array of day-by-day dates that will be displayed on chart according to given date range.
        const datesArr = new Array<Date>();
        const dt = new Date(since);
        dt.setHours(0, 0, 0, 0);

        // Fill previously created array with day-by-day dates.
        while (dt.getTime() <= before.getTime()) {
            datesArr.push(new Date(dt));
            dt.setDate(dt.getDate() + 1);
        }

        // Create new array of objects with date and corresponding data value with length of date range difference.
        const chartData: DataStamp[] = new Array(datesArr.length);

        // Fill new array.
        for (let i = 0; i < datesArr.length; i++) {
            // Find in fetched data a day-data value that corresponds to current iterable date.
            const foundData = fetchedData.find(el => el.intervalStart.getTime() === datesArr[i].getTime())
            // If found then fill new array with appropriate day-data value.
            if (foundData) {
                chartData[i] = foundData;
                continue;
            }

            // If not found then fill new array with day and zero data value.
            chartData[i] = DataStamp.emptyWithDate(datesArr[i]);
        }

        return chartData;
    }
}

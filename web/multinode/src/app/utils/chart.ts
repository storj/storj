// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BandwidthRollup } from '@/bandwidth';
import { SizeBreakpoints } from '@/private/memory/size';
import { Stamp } from '@/storage';

const shortMonthNames = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sept', 'Oct', 'Nov', 'Dec'];

/**
 * Used to display correct and convenient data on chart.
 */
export class Chart {
    /**
     * Brings chart data to a more compact form.
     * @param data - holds array of chart data in numeric form
     * @returns data - numeric array of normalized data
     */
    public static normalizeChartData(data: number[]): number[] {
        const maxBytes = Math.ceil(Math.max(...data));

        let divider: number = SizeBreakpoints.PB;

        switch (true) {
        case maxBytes < SizeBreakpoints.MB:
            divider = SizeBreakpoints.KB;
            break;
        case maxBytes < SizeBreakpoints.GB:
            divider = SizeBreakpoints.MB;
            break;
        case maxBytes < SizeBreakpoints.TB:
            divider = SizeBreakpoints.GB;
            break;
        case maxBytes < SizeBreakpoints.PB:
            divider = SizeBreakpoints.TB;
            break;
        }

        return data.map(elem => elem / divider);
    }

    /**
     * gets chart data dimension depending on data size.
     * @param data - holds array of chart data in numeric form
     * @returns dataDimension - string of data dimension
     */
    public static getChartDataDimension(data: number[]): string {
        const maxBytes = Math.ceil(Math.max(...data));

        let dataDimension: string;

        switch (true) {
        case maxBytes < SizeBreakpoints.MB:
            dataDimension = 'KB';
            break;
        case maxBytes < SizeBreakpoints.GB:
            dataDimension = 'MB';
            break;
        case maxBytes < SizeBreakpoints.TB:
            dataDimension = 'GB';
            break;
        case maxBytes < SizeBreakpoints.PB:
            dataDimension = 'TB';
            break;
        default:
            dataDimension = 'PB';
        }

        return dataDimension;
    }

    /**
     * Used to display correct number of days on chart's labels.
     *
     * @returns daysDisplayed - array of days converted to a string by using the current or specified locale
     */
    public static daysDisplayedOnChart(): string[] {
        const daysDisplayed = Array<string>(new Date().getUTCDate());
        const currentMonth = shortMonthNames[new Date().getUTCMonth()].toUpperCase();

        for (let i = 0; i < daysDisplayed.length; i++) {
            daysDisplayed[i] = `${currentMonth} ${i + 1}`;
        }

        if (daysDisplayed.length === 1) {
            daysDisplayed.unshift('0');
        }

        return daysDisplayed;
    }

    /**
     * Adds missing bandwidth usage for bandwidth chart data for each day of month.
     * @param fetchedData - array of data that is spread over missing bandwidth usage for each day of the month
     * @returns bandwidthChartData - array of filled data
     */
    public static populateEmptyBandwidth(fetchedData: BandwidthRollup[]): BandwidthRollup[] {
        const bandwidthChartData: BandwidthRollup[] = new Array(new Date().getUTCDate());
        const data: BandwidthRollup[] = fetchedData || [];

        if (data.length === 0) {
            return bandwidthChartData;
        }

        outer:
        for (let i = 0; i < bandwidthChartData.length; i++) {
            const date = i + 1;

            for (let j = 0; j < data.length; j++) {
                if (data[j].intervalStart.getUTCDate() === date) {
                    bandwidthChartData[i] = data[j];
                    continue outer;
                }
            }

            bandwidthChartData[i] = BandwidthRollup.emptyWithDate(date);
        }

        if (bandwidthChartData.length === 1) {
            bandwidthChartData.unshift(BandwidthRollup.emptyWithDate(1));
            bandwidthChartData[0].intervalStart.setUTCHours(0, 0, 0, 0);
        }

        return bandwidthChartData;
    }

    /**
     * Adds missing stamps for storage chart data for each day of month.
     * @param fetchedData - array of data that is spread over missing stamps for each day of the month
     * @returns storageChartData - array of filled data
     */
    public static populateEmptyStamps(fetchedData: Stamp[]): Stamp[] {
        const storageChartData: Stamp[] = new Array(new Date().getUTCDate());
        const data: Stamp[] = fetchedData || [];

        if (data.length === 0) {
            return storageChartData;
        }

        outer:
        for (let i = 0; i < storageChartData.length; i++) {
            const date = i + 1;

            for (let j = 0; j < data.length; j++) {
                if (data[j].intervalStart.getUTCDate() === date) {
                    storageChartData[i] = data[j];
                    continue outer;
                }
            }

            storageChartData[i] = Stamp.emptyWithDate(date);
        }

        if (storageChartData.length === 1) {
            storageChartData.unshift(Stamp.emptyWithDate(1));
            storageChartData[0].intervalStart.setUTCHours(0, 0, 0, 0);
        }

        return storageChartData;
    }
}

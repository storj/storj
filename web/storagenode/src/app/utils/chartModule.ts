// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BandwidthUsed, Stamp } from '@/storagenode/satellite';

export class ChartFormatter {
    // createBandwidthChartItems creates new BandwidthChartItem array from start of the month till today
    // and fills it with fetched data if there is data for given date, or creates empty item otherwise
    public static populateEmptyBandwidth(fetchedData: BandwidthUsed[]): BandwidthUsed[] {
        const bandwidthChartData: BandwidthUsed[] = new Array(new Date().getDate());
        const data: BandwidthUsed[] = fetchedData ? fetchedData : [];

        outer:
        for (let i = 0; i < bandwidthChartData.length; i++) {
            const date = i + 1;

            for (let j = 0; j < data.length; j++) {
                const fetched = data[j];

                if (fetched.from.getDate() === date) {
                    bandwidthChartData[i] = fetched;
                    continue outer;
                }
            }

            bandwidthChartData[i] = BandwidthUsed.emptyWithDate(date);
        }

        return bandwidthChartData;
    }

    /**
     * adds missing stamps to have stamps for each day of month
     * @param fetchedData
     */
     public static populateEmptyStamps(fetchedData: Stamp[]): Stamp[] {
        const result: Stamp[] = new Array(new Date().getDate());
        const data: Stamp[] = fetchedData ? fetchedData : [];

        if (data.length === 0) {
            return result;
        }

        outer:
        for (let i = 0; i < result.length; i++ ) {
            const date = i + 1;

            for (let j = 0; j < data.length; j++) {
                if (data[j].timestamp.getDate() === date) {
                    result[i] = data[j];
                    continue outer;
                }

                result[i] = Stamp.emptyWithDate(date);
            }
        }

        return result;
    }
}

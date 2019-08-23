// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { GB, KB, MB } from '@/app/utils/converter';

export class ChartUtils {
    public static normalizeArray(data: number[]): number[] {
        const maxBytes = Math.ceil(Math.max(...data));

        let divider: number = GB;
        switch (true) {
            case maxBytes < MB:
                divider = KB;
                break;
            case maxBytes < GB:
                divider = MB;
                break;
        }

        return data.map(elem => elem / divider);
    }

    public static xAxeOptions(date: Date): string[] {
        let daysDisplayed = Array(date.getDate());

        for (let i = 0; i < daysDisplayed.length; i++) {
            let date = new Date();
            date.setDate(i + 1);

            daysDisplayed[i] = date.toLocaleDateString('en-US', {day: 'numeric'});
        }

        return daysDisplayed;
    }
}

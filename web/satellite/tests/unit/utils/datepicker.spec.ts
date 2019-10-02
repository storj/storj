// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    DateGenerator, DateStamp, DayItem
} from '@/utils/datepicker';

describe('datepicker', () => {
    it('DateGenerator populate years correctly', () => {
        const dateGenerator = new DateGenerator();
        const currentYear = new Date().getFullYear();

        const years = dateGenerator.populateYears();

        expect(years.length).toBe(100);
        expect(years[0]).toBe(currentYear);
    });

    it('DateGenerator populate days correctly with exact date and isSundayFirst', () => {
        const dateGenerator = new DateGenerator();
        // 8th month is september
        const currentDate = new DateStamp(2019, 8, 30);
        const firstExpectedDay = new DayItem(
            25,
            false,
            false,
            false,
            new Date(2019, 7, 25),
            1,
            false,
        );
        const lastExpectedDay = new DayItem(
            5,
            false,
            false,
            false,
            new Date(2019, 9, 5),
            1,
            false,
        );

        const days = dateGenerator.populateDays(currentDate, true);

        expect(days.length).toBe(42);
        expect(days[0].equals(firstExpectedDay.moment)).toBe(true);
        expect(days[days.length - 1].equals(lastExpectedDay.moment)).toBe(true);
    });

    it('DateGenerator populate days correctly with exact date and no isSundayFirst', () => {
        const dateGenerator = new DateGenerator();
        // 8th month is september
        const currentDate = new DateStamp(2019, 8, 30);
        const firstExpectedDay = new DayItem(
            26,
            false,
            false,
            false,
            new Date(2019, 7, 26),
            1,
            false,
        );
        const lastExpectedDay = new DayItem(
            6,
            false,
            false,
            false,
            new Date(2019, 9, 6),
            1,
            false,
        );

        const days = dateGenerator.populateDays(currentDate, false);

        expect(days.length).toBe(42);
        expect(days[0].equals(firstExpectedDay.moment)).toBe(true);
        expect(days[days.length - 1].equals(lastExpectedDay.moment)).toBe(true);
    });
});

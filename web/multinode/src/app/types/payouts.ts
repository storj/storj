// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { monthNames } from '@/app/types/date';

/**
 * Describes month button entity for calendar.
 */
export class MonthButton {
    public constructor(
        public year: number = 0,
        public index: number = 0,
        public active: boolean = false,
        public selected: boolean = false,
    ) {}

    /**
     * Returns month label depends on index.
     */
    public get name(): string {
        return monthNames[this.index] ? monthNames[this.index].slice(0, 3) : '';
    }
}

export interface StoredMonthsByYear {
    [key: number]: MonthButton[];
}

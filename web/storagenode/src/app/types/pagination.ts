// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Describes callback for pagination page click.
 */
export type OnPageClickCallback = (index: number) => Promise<void>;

export type CheckSelected = (index: number) => boolean;

/**
 * Describes page item in paginator.
 */
export class Page {
    constructor(
        public index: number = 1,
        public onClick: OnPageClickCallback,
    ) {}

    public async select(): Promise<void> {
        await this.onClick(this.index);
    }
}

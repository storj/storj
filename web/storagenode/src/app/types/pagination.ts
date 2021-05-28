// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Describes callback for pagination page click.
 */
export type OnPageClickCallback = (index: number) => Promise<any>;

export type CheckSelected = (index: number) => boolean;

/**
 * Describes page item in paginator.
 */
export class Page {
    private readonly pageIndex: number = 1;
    private readonly onClick: OnPageClickCallback;

    constructor(index: number, callback: OnPageClickCallback) {
        this.pageIndex = index;
        this.onClick = callback;
    }

    public get index() {
        return this.pageIndex;
    }

    public async select(): Promise<void> {
        await this.onClick(this.pageIndex);
    }
}

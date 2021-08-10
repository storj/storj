// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export type OnPageClickCallback = (pageNumber: number) => Promise<void>;

export type CheckSelected = (index: number) => boolean;

/**
 * Describes paginator page.
 */
export class Page {
    private readonly pageNumber: number = 1;
    private readonly onClick: OnPageClickCallback;

    constructor(pageNumber: number, callback: OnPageClickCallback) {
        this.pageNumber = pageNumber;
        this.onClick = callback;
    }

    public get index(): number {
        return this.pageNumber;
    }

    public async select(): Promise<void> {
        await this.onClick(this.pageNumber);
    }
}

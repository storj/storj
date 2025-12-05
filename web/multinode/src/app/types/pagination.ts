// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export type OnPageClickCallback = (pageNumber: number) => Promise<void>;

export type CheckSelected = (index: number) => boolean;

/**
 * Describes paginator page.
 */
export class Page {
    constructor(
        public pageNumber: number,
        public onClick: OnPageClickCallback,
    ) {}

    public get index(): number {
        return this.pageNumber;
    }

    public async select(): Promise<void> {
        await this.onClick(this.pageNumber);
    }
}

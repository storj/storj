// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

// Cursor holds cursor entity which is used to create listed page.
export class Cursor {
    public constructor(
        public limit: number,
        public page: number,
    ) {}
}

// Page holds page entity which is used to show listed page.
export class Page<T> {
    public constructor(
        public items: T[] = [],
        public offset: number = 0,
        public limit: number = 0,
        public currentPage: number = 1,
        public pageCount: number = 0,
        public totalCount: number = 0,
    ) {}
}

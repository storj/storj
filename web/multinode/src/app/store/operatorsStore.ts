// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { reactive } from 'vue';

import { Operator, Operators } from '@/operators';
import { Operators as OperatorsClient } from '@/api/operators';
import { Cursor } from '@/private/pagination';

class OperatorsState {
    public constructor(
        public operators: Operator[] = [],
        public limit: number = 2,
        public currentPage: number = 1,
        public pageCount: number = 0,
        public totalCount: number = 0,
    ) {}
}

export const useOperatorsStore = defineStore('operators', () => {
    const state = reactive(new OperatorsState());

    const service = new Operators(new OperatorsClient());

    async function listPaginated(pageNumber: number): Promise<void> {
        const cursor: Cursor = new Cursor(state.limit, pageNumber);
        const page = await service.listPaginated(cursor);

        state.operators = page.items;
        state.limit = page.limit;
        state.currentPage = page.currentPage;
        state.pageCount = page.pageCount;
        state.totalCount = page.totalCount;
    }

    return {
        state,
        listPaginated,
    };
});

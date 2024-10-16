// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { ActionContext, ActionTree, GetterTree, Module, MutationTree } from 'vuex';

import { RootState } from '@/app/store/index';
import { Operator, Operators } from '@/operators';
import { Cursor, Page } from '@/private/pagination';

/**
 * OperatorsState is a representation of operators module state.
 */
export class OperatorsState {
    public constructor(
        public operators: Operator[] = [],
        public limit: number = 5,
        public currentPage: number = 1,
        public pageCount: number = 0,
        public totalCount: number = 0,
    ) {}
}

/**
 * OperatorsModule is a part of a global store that encapsulates all operators related logic.
 */
export class OperatorsModule implements Module<OperatorsState, RootState> {
    public readonly namespaced: boolean;
    public readonly state: OperatorsState;
    public readonly getters?: GetterTree<OperatorsState, RootState>;
    public readonly actions: ActionTree<OperatorsState, RootState>;
    public readonly mutations: MutationTree<OperatorsState>;

    private readonly operators: Operators;

    public constructor(operators: Operators) {
        this.operators = operators;

        this.namespaced = true;
        this.state = new OperatorsState();

        this.mutations = {
            populate: this.populate,
            updateCurrentPage: this.updateCurrentPage,
        };

        this.actions = {
            listPaginated: this.listPaginated.bind(this),
        };
    }

    /**
     * populate mutation will set state with new operators array.
     * @param state - state of the operators module.
     * @param page - holds page which is used to show operators listed page.
     */
    public populate(state: OperatorsState, page: Page<Operator>): void {
        state.operators = page.items;
        state.limit = page.limit;
        state.currentPage = page.currentPage;
        state.pageCount = page.pageCount;
        state.totalCount = page.totalCount;
    }

    /**
     * updates current page.
     * @param state - state of the operators module.
     * @param pageNumber - desired page number.
     */
    public updateCurrentPage(state: OperatorsState, pageNumber: number): void {
        state.currentPage = pageNumber;
    }

    /**
     * listPaginated action loads page with operators.
     * @param ctx - context of the Vuex action.
     * @param pageNumber - number of page to get.
     */
    public async listPaginated(ctx: ActionContext<OperatorsState, RootState>, pageNumber: number): Promise<void> {
        const cursor: Cursor = new Cursor(ctx.state.limit, pageNumber);
        const page = await this.operators.listPaginated(cursor);

        ctx.commit('updateCurrentPage', pageNumber);
        ctx.commit('populate', page);
    }
}

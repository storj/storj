// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

type Mutation<State> =
    (state: State, ...args: any[]) => any; // eslint-disable-line @typescript-eslint/no-explicit-any

type Action<Context> =
    (context: Context, ...args: any[]) => (Promise<any> | void | any); // eslint-disable-line @typescript-eslint/no-explicit-any

type Getter<State, Context> =
    Context extends {rootGetters: any} ? ( // eslint-disable-line @typescript-eslint/no-explicit-any
        ((state: State) => any) | // eslint-disable-line @typescript-eslint/no-explicit-any
        ((state: State, rootGetters: Context['rootGetters']) => any) // eslint-disable-line @typescript-eslint/no-explicit-any
        ) : ((state: State) => any); // eslint-disable-line @typescript-eslint/no-explicit-any

export interface StoreModule<State, Context> { // eslint-disable-line @typescript-eslint/no-unused-vars
    state: State;
    mutations: Record<string, Mutation<State>>
    actions: Record<string, Action<Context>>
    getters?: Record<string, Getter<State, Context>>
}
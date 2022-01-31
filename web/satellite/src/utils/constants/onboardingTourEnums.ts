// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

export enum AddingPaymentState {
    ADD_CARD = 1,
    ADD_STORJ,
}

export enum AddingStorjState {
    DEFAULT = 1,
    VERIFYING,
    VERIFIED,
}

// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { KindInfo } from '@/api/client.gen';

export enum UserKind {
    Free, Paid, NFR,
}

export function userIsPaid(user: { kind: KindInfo }): boolean {
    return user.kind.value === UserKind.Paid;
}

export function userIsFree(user: { kind: KindInfo }): boolean {
    return user.kind.value === UserKind.Free;
}

export function userIsNFR(user: { kind: KindInfo }): boolean {
    return user.kind.value === UserKind.NFR;
}

export enum UserStatus {
    Inactive = 0,
    Active = 1,
    Deleted = 2,
    PendingDeletion = 3,
    LegalHold = 4,
    PendingBotVerification = 5,
    UserRequestedDeletion = 6,
}
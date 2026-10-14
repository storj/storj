// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { KindInfo, UserStatusInfo } from '@/api/client.gen';

export enum UserKind {
    Free, Paid, NFR, Member,
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

export function userKindColor(user: { kind: KindInfo }): string {
    if (userIsPaid(user)) return 'success';
    if (userIsNFR(user)) return 'warning';
    return 'info';
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

export function userStatusColor(status: UserStatusInfo): string {
    switch (status.value) {
    case UserStatus.Active:
        return 'success';
    case UserStatus.Deleted:
    case UserStatus.PendingDeletion:
    case UserStatus.UserRequestedDeletion:
        return 'error';
    default:
        return 'warning';
    }
}

// Freeze status filters of the accounts list besides the freeze event types.
export const FREEZE_STATUS_NONE = 'none';
export const FREEZE_STATUS_ANY = 'any';

export enum AccountsOrderBy {
    CreatedAt = 'createdAt',
    Email = 'email',
    Name = 'name',
}

export type AccountsOrderDirection = 'asc' | 'desc';

export class AccountsListCursor {
    public page = 1;
    public limit = 25;
    public status: number | null = null;
    public kind: number | null = null;
    // FREEZE_STATUS_NONE, FREEZE_STATUS_ANY or the value of a freeze event type.
    public freezeStatus: string | null = null;
    public order: AccountsOrderBy = AccountsOrderBy.CreatedAt;
    public direction: AccountsOrderDirection = 'desc';
}

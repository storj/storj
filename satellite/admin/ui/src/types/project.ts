// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

export enum ProjectStatus {
    Disabled = 0,
    Active = 1,
    PendingDeletion = 2,
}

export interface MembersCursor {
    search: string;
    limit: number;
    page: number;
    order: ProjectMemberOrderBy;
    orderDirection: ProjectMemberOrderDirection;
}

export enum ProjectMemberOrderBy {
    EMAIL = 1,
    DATE,
}

export enum ProjectMemberOrderDirection {
    ASC = 1,
    DESC = 2,
}

export enum ProjectMemberRole {
    ADMIN = 0,
    MEMBER = 1,
}

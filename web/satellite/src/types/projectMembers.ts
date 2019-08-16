// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// ProjectMember stores needed info about user info to show it on UI
import { User } from '@/types/users';
import { SortDirection } from '@/types/common';

export const firstPage = 1;

export type OnHeaderClickCallback = (sortBy: ProjectMemberOrderBy, sortDirection: SortDirection) => Promise<any>;

export enum ProjectMemberOrderBy {
    NAME = 1,
    EMAIL,
    CREATED_AT,
}

export class ProjectMemberCursor {
    public search: string;
    public limit: number;
    public page: number;
    public order: ProjectMemberOrderBy;
    public orderDirection: SortDirection;

    public constructor() {
        this.search = '';
        this.limit = 6;
        this.page = 1;
        this.order = ProjectMemberOrderBy.NAME;
        this.orderDirection = SortDirection.ASCENDING;
    }
}

export class ProjectMembersPage {
    public projectMembers: ProjectMember[];
    public search: string;
    public order: ProjectMemberOrderBy;
    public orderDirection: SortDirection;
    public limit: number;
    public pageCount: number;
    public currentPage: number;
    public totalCount: number;

    public constructor() {
        this.projectMembers = [];
        this.search = '';
        this.order = ProjectMemberOrderBy.NAME;
        this.orderDirection = SortDirection.ASCENDING;
        this.limit = 8;
        this.pageCount = 0;
        this.currentPage = 1;
        this.totalCount = 0;
    }
}

export class ProjectMember {
    public user: User;

    public joinedAt: string;
    public isSelected: boolean;

    public constructor(fullName: string, shortName: string, email: string, joinedAt: string, id?: string) {
        this.user = new User(id || '', fullName, shortName, email);
        this.joinedAt = joinedAt;
    }

    public formattedFullName(): string {
        let fullName: string = this.user.getFullName();

        if (fullName.length > 16) {
            fullName = fullName.slice(0, 13) + '...';
        }

        return fullName;
    }

    public formattedEmail(): string {
        let email: string = this.user.email;

        if (email.length > 16) {
            email = this.user.email.slice(0, 13) + '...';
        }

        return email;
    }

    public joinedAtLocal(): string {
        if (!this.joinedAt) return '';

        return new Date(this.joinedAt).toLocaleDateString();
    }
}


// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// ProjectMember stores needed info about user info to show it on UI
import { User } from '@/types/users';
import { ProjectMemberSortByEnum, ProjectMemberSortDirectionEnum } from '@/utils/constants/ProjectMemberSortEnum';

export type OnHeaderClickCallback = (sortBy: ProjectMemberSortByEnum, sortDirection: ProjectMemberSortDirectionEnum) => Promise<any>;

export class ProjectMemberCursor {
    public search: string;
    public limit: number;
    public page: number;
    public order: ProjectMemberSortByEnum;
    public orderDirection: ProjectMemberSortDirectionEnum;

    public constructor() {
        this.search = '';
        this.limit = 6;
        this.page = 1;
        this.order = ProjectMemberSortByEnum.NAME;
        this.orderDirection = ProjectMemberSortDirectionEnum.ASCENDING;
    }
}

export class ProjectMembersPage {
    public projectMembers: ProjectMember[];
    public search: string;
    public order: ProjectMemberSortByEnum;
    public orderDirection: ProjectMemberSortDirectionEnum;
    public limit: number;
    public pageCount: number;
    public currentPage: number;
    public totalCount: number;

    public constructor() {
        this.projectMembers = [];
        this.search = '';
        this.order = ProjectMemberSortByEnum.NAME;
        this.orderDirection = ProjectMemberSortDirectionEnum.ASCENDING;
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
        this.user = new User(fullName, shortName, email);
        this.user.id = id || '';
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


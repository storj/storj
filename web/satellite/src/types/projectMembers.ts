// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// ProjectMember stores needed info about user info to show it on UI
import { SortDirection } from '@/types/common';
import { User } from '@/types/users';

export type OnHeaderClickCallback = (sortBy: ProjectMemberOrderBy, sortDirection: SortDirection) => Promise<void>;

export enum ProjectMemberOrderBy {
    NAME = 1,
    EMAIL,
    CREATED_AT,
}

/**
 * Contains values of project members header component state
 * used in ProjectMembersArea and HeaderArea.
 */
export enum ProjectMemberHeaderState {
    DEFAULT = 0,
    /**
     * Used when some project members selected
     */
    ON_SELECT,
}

/**
 * ProjectMembersApi is a graphql implementation of ProjectMembers API.
 * Exposes all ProjectMembers-related functionality
 */
export interface ProjectMembersApi {
    /**
     * Add members to project by user emails.
     *
     * @param projectId
     * @param emails list of project members email to add
     *
     * @throws Error
     */
    add(projectId: string, emails: string[]): Promise<void>;

    /**
     * Deletes ProjectMembers from project by project member emails
     *
     * @param projectId
     * @param emails
     *
     * @throws Error
     */
    delete(projectId: string, emails: string[]): Promise<void>;

    /**
     * Fetch Project Members
     *
     * @param projectId
     * @param cursor
     *
     * @throws Error
     */
    get(projectId: string, cursor: ProjectMemberCursor): Promise<ProjectMembersPage>;
}

// ProjectMemberCursor is a type, used for paged project members request
export class ProjectMemberCursor {
    public constructor(
        public search: string = '',
        public limit: number = 6,
        public page: number = 1,
        public order: ProjectMemberOrderBy = ProjectMemberOrderBy.NAME,
        public orderDirection: SortDirection = SortDirection.ASCENDING) {
    }
}

// ProjectMembersPage is a type, used to describe paged project members list
export class ProjectMembersPage {
    public constructor(
        public projectMembers: ProjectMember[] = [],
        public search: string = '',
        public order: ProjectMemberOrderBy = ProjectMemberOrderBy.NAME,
        public orderDirection: SortDirection = SortDirection.ASCENDING,
        public limit: number = 6,
        public pageCount: number = 0,
        public currentPage: number = 1,
        public totalCount: number = 0) {
    }
}

// ProjectMember is a type, used to describe project member
export class ProjectMember {
    public user: User;

    public joinedAt: string;
    public isSelected: boolean;

    public constructor(fullName: string, shortName: string, email: string, joinedAt: string, id?: string) {
        this.user = new User(id || '', fullName, shortName, email);
        this.joinedAt = joinedAt;
        this.isSelected = false;
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

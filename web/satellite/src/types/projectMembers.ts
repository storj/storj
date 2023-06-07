// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * ProjectMember stores needed info about user info to show it on UI.
 */
import { SortDirection } from '@/types/common';
import { User } from '@/types/users';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';

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
     * Invite members to project by user emails.
     *
     * @param projectId
     * @param emails list of project members email to add
     *
     * @throws Error
     */
    invite(projectId: string, emails: string[]): Promise<void>;

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

/**
 * ProjectMemberCursor is a type, used for paged project members request.
 */
export class ProjectMemberCursor {
    public constructor(
        public search: string = '',
        public limit: number = DEFAULT_PAGE_LIMIT,
        public page: number = 1,
        public order: ProjectMemberOrderBy = ProjectMemberOrderBy.NAME,
        public orderDirection: SortDirection = SortDirection.ASCENDING,
    ) {}
}

/**
 * ProjectMembersPage is a type, used to describe paged project members list.
 */
export class ProjectMembersPage {
    public constructor(
        public projectMembers: ProjectMember[] = [],
        public search: string = '',
        public order: ProjectMemberOrderBy = ProjectMemberOrderBy.NAME,
        public orderDirection: SortDirection = SortDirection.ASCENDING,
        public limit: number = 6,
        public pageCount: number = 0,
        public currentPage: number = 1,
        public totalCount: number = 0,
    ) {}
}

/**
 * ProjectMember is a type, used to describe project member.
 */
export class ProjectMember {
    public user: User;
    public isSelected: boolean;

    public constructor(
        public fullName: string = '',
        public shortName: string = '',
        public email: string = '',
        public joinedAt: Date = new Date(),
        public id: string = '',
    ) {
        this.user = new User(this.id, this.fullName, this.shortName, this.email);
        this.isSelected = false;
    }

    /**
     * Returns user's full name.
     */
    public get name(): string {
        return this.user.getFullName();
    }

    /**
     * Returns joined at date as a local date string.
     */
    public localDate(): string {
        return this.joinedAt.toLocaleDateString('en-US', { day:'numeric', month:'short', year:'numeric' });
    }
}

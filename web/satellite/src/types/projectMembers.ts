// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * ProjectMember stores needed info about user info to show it on UI.
 */
import { SortDirection } from '@/types/common';
import { User } from '@/types/users';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';

export enum ProjectMemberOrderBy {
    NAME = 1,
    EMAIL,
    DATE,
}

/**
 * ProjectMembersApi exposes all ProjectMembers-related functionality
 */
export interface ProjectMembersApi {

    /**
     * Invites a user to a project.
     *
     * @param projectId
     * @param email email of the project member to add
     * @param csrfProtectionToken CSRF token
     *
     * @throws Error
     */
    invite(projectId: string, email: string, csrfProtectionToken: string): Promise<void>;

    /**
     * Resends invitations to pending project members.
     *
     * @param projectId
     * @param emails emails of the project members whose invitations should be resent
     * @param csrfProtectionToken CSRF token
     *
     * @throws Error
     */
    reinvite(projectId: string, emails: string[], csrfProtectionToken: string): Promise<void>;

    /**
     * Used for fetching team member related to project.
     *
     * @param projectID
     * @param memberID
     *
     * @throws Error
     */
    getSingleMember(projectID: string, memberID: string): Promise<ProjectMember>

    /**
     * Handles updating project member's role.
     *
     * @param projectID
     * @param memberID
     * @param role
     * @param csrfProtectionToken CSRF token
     *
     * @throws Error
     */
    updateRole(projectID: string, memberID: string, role: ProjectRole, csrfProtectionToken: string): Promise<ProjectMember>

    /**
     * Get invite link for the specified project and email.
     *
     * @param projectId
     * @param email
     *
     * @throws Error
     */
    getInviteLink(projectId: string, email: string): Promise<string>;

    /**
     * Deletes ProjectMembers from project by project member emails
     *
     * @param projectId
     * @param emails
     * @param csrfProtectionToken
     *
     * @throws Error
     */
    delete(projectId: string, emails: string[], csrfProtectionToken: string): Promise<void>;

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
        public orderDirection: SortDirection = SortDirection.asc,
    ) {}
}

/**
 * ProjectMembersPage is a type, used to describe paged project members list.
 */
export class ProjectMembersPage {
    public constructor(
        public projectMembers: ProjectMember[] = [],
        public projectInvitations: ProjectInvitationItemModel[] = [],
        public search: string = '',
        public order: ProjectMemberOrderBy = ProjectMemberOrderBy.NAME,
        public orderDirection: SortDirection = SortDirection.asc,
        public limit: number = 6,
        public pageCount: number = 0,
        public currentPage: number = 1,
        public totalCount: number = 0,
    ) {}

    /**
     * Returns all project members and invitations as ProjectMemberItemModel.
     */
    public getAllItems(): ProjectMemberItemModel[] {
        const items = (this.projectMembers as ProjectMemberItemModel[]).concat(this.projectInvitations);
        return items.sort((a, b) => {
            let cmp: (a: ProjectMemberItemModel, b: ProjectMemberItemModel) => number;

            if (this.order === ProjectMemberOrderBy.DATE) {
                cmp = (a, b) => a.getJoinDate().getTime() - b.getJoinDate().getTime();
            } else {
                cmp = (a, b) => a.getName().toLowerCase().localeCompare(b.getName().toLowerCase());
            }

            const result = (this.orderDirection === SortDirection.desc) ? cmp(b, a) : cmp(a, b);
            return (result !== 0) ? result : a.getEmail().toLowerCase().localeCompare(b.getEmail().toLowerCase());
        });
    }
}

/**
 * ProjectInvitationItemModel represents the view model for project member list items.
 */
export interface ProjectMemberItemModel {
    /**
     * Returns the member's user ID if it exists.
     */
    getUserID(): string | undefined;

    /**
     * Returns the member's name.
     */
    getName(): string;

    /**
     * Returns the member's email address.
     */
    getEmail(): string,

    /**
     * Returns the date that the member joined the project.
     */
    getJoinDate(): Date;

    /**
     * Returns the member's role.
     */
    getRole(): ProjectRole;

    /**
     * Sets whether the member item has been selected.
     */
    setSelected(selected: boolean): void;

    /**
     * Returns whether the member item has been selected.
     */
    isSelected(): boolean;

    /**
     * Returns whether the member has yet to accept its invitation.
     */
    isPending(): boolean;
}

/**
 * ProjectMember is a type, used to describe project member.
 */
export class ProjectMember implements ProjectMemberItemModel {
    public user: User;
    public role: ProjectRole;
    public _isSelected = false;

    public constructor(
        public fullName: string = '',
        public shortName: string = '',
        public email: string = '',
        public joinedAt: Date = new Date(),
        public id: string = '',
        private _role: number = 0,
    ) {
        this.user = new User(this.id, '', this.fullName, this.shortName, this.email);
        this.setRole();
    }

    /**
     * Returns the user's ID.
     */
    public getUserID(): string | undefined {
        return this.id;
    }

    /**
     * Returns user's full name.
     */
    public getName(): string {
        return this.user.getFullName();
    }

    /**
     * Returns user's email address.
     */
    public getEmail(): string {
        return this.email;
    }

    /**
     * Returns the date that the member joined the project.
     */
    public getJoinDate(): Date {
        return this.joinedAt;
    }

    /**
     * Returns project member role.
     */
    public getRole(): ProjectRole {
        return this.role;
    }

    /**
     * Sets whether the member item has been selected.
     */
    public setSelected(selected: boolean): void {
        this._isSelected = selected;
    }

    /**
     * Returns whether the member item has been selected.
     */
    public isSelected(): boolean {
        return this._isSelected;
    }

    /**
     * Returns whether the member has yet to accept its invitation.
     * Always false. Required for implementing ProjectMemberItemModel.
     */
    public isPending(): boolean {
        return false;
    }

    private setRole(): void {
        switch (this._role) {
        case 1:
            this.role = ProjectRole.Member;
            break;
        default:
            this.role = ProjectRole.Admin;
        }
    }
}

/**
 * ProjectInvitationItemModel represents the view model for project member invitation list items.
 */
export class ProjectInvitationItemModel implements ProjectMemberItemModel {
    private _isSelected = false;

    public constructor(
        public email: string,
        public createdAt: Date,
        public expired: boolean,
    ) {}

    /**
     * Returns a null user ID. Required for implementing ProjectMemberItemModel.
     */
    public getUserID(): string | undefined {
        return undefined;
    }

    /**
     * Returns the placeholder name of the invitation recipient.
     */
    public getName(): string {
        return this.getEmail().split('@')[0];
    }

    /**
     * Returns the invitation recipient's email address.
     */
    public getEmail(): string {
        return this.email.toLowerCase();
    }

    /**
     * Returns the date that the invitation was created.
     */
    public getJoinDate(): Date {
        return this.createdAt;
    }

    /**
     * Returns project member role.
     */
    public getRole(): ProjectRole {
        return ProjectRole.Member;
    }

    /**
     * Sets whether the invitation item has been selected.
     */
    public setSelected(selected: boolean): void {
        this._isSelected = selected;
    }

    /**
     * Returns whether the invitation item has been selected.
     */
    public isSelected(): boolean {
        return this._isSelected;
    }

    /**
     * Returns whether the member has yet to accept its invitation.
     * Always true. Required for implementing ProjectMemberItemModel.
     */
    public isPending(): boolean {
        return true;
    }
}

/**
 * ProjectRole represents a project member's role.
 */
export enum ProjectRole {
    Admin = 'Admin',
    Member = 'Member',
    Owner = 'Owner',
    Invited = 'Invited',
    InviteExpired = 'Invite Expired',
}

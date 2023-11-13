// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';

/**
 * Exposes all project-related functionality.
 */
export interface ProjectsApi {
    /**
     * Creates project.
     *
     * @param createProjectFields - contains project information
     * @throws Error
     */
    create(createProjectFields: ProjectFields): Promise<Project>;
    /**
     * Fetch projects.
     *
     * @returns Project[]
     * @throws Error
     */
    get(): Promise<Project[]>;
    /**
     * Update project name and description.
     *
     * @param projectId - project ID
     * @param updateProjectFields - project fields to update
     * @param updateProjectLimits - project limits to update
     * @returns Project[]
     * @throws Error
     */
    update(projectId: string, updateProjectFields: ProjectFields, updateProjectLimits: ProjectLimits): Promise<void>;

    /**
     * Get project limits.
     *
     * @param projectId - project ID
     * @throws Error
     */
    getLimits(projectId: string): Promise<ProjectLimits>;

    /**
     * Request limit increase.
     *
     * @param projectId - project ID
     * @param info - request information
     * @throws Error
     */
    requestLimitIncrease(projectId: string, info: LimitRequestInfo): Promise<void>;

    /**
     * Get project salt
     *
     * @param projectID - project ID
     * @throws Error
     */
    getSalt(projectID: string): Promise<string>;

    /**
     * Get project limits.
     *
     * @throws Error
     */
    getTotalLimits(): Promise<ProjectLimits>;

    /**
     * Get link to download total usage report for all the projects that user owns.
     *
     * @throws Error
     */
    getTotalUsageReportLink(start: Date, end: Date, projectID: string): string

    /**
     * Get project daily usage by specific date range.
     *
     * @throws Error
     */
    getDailyUsage(projectID: string, start: Date, end: Date): Promise<ProjectsStorageBandwidthDaily>;

    /**
     * Fetch owned projects.
     *
     * @returns ProjectsPage
     * @throws Error
     */
    getOwnedProjects(cursor: ProjectsCursor): Promise<ProjectsPage>;

    /**
     * Returns a user's pending project member invitations.
     *
     * @throws Error
     */
    getUserInvitations(): Promise<ProjectInvitation[]>;

    /**
     * Handles accepting or declining a user's project member invitation.
     *
     * @throws Error
     */
    respondToInvitation(projectID: string, response: ProjectInvitationResponse): Promise<void>;
}

/**
 * MAX_NAME_LENGTH defines maximum amount of symbols for project name.
 */
export const MAX_NAME_LENGTH = 20;

/**
 * MAX_DESCRIPTION_LENGTH defines maximum amount of symbols for project description.
 */
export const MAX_DESCRIPTION_LENGTH = 100;

/**
 * Project is a type, used for creating new project in backend.
 */
export class Project {
    public urlId: string;

    public constructor(
        public id: string = '',
        public name: string = '',
        public description: string = '',
        public createdAt: string = '',
        public ownerId: string = '',
        public isSelected: boolean = false,
        public memberCount: number = 0,
        public edgeURLOverrides?: EdgeURLOverrides,
    ) {}

    /**
     * Returns created date as a local string.
     */
    public createdDate(): string {
        const createdAt = new Date(this.createdAt);
        return createdAt.toLocaleString('en-US', { year: 'numeric', month: '2-digit', day: 'numeric' });
    }
}

/**
 * EdgeURLOverrides contains overrides for edge service URLs.
 */
export type EdgeURLOverrides = {
    authService?: string;
    publicLinksharing?: string;
    internalLinksharing?: string;
};

/**
 * ProjectFields is a type, used for creating and updating project.
 */
export class ProjectFields {
    public constructor(
        public name: string = '',
        public description: string = '',
        public ownerId: string = '',
    ) {}

    /**
     * checkName checks if project name is valid.
     */
    public checkName(): void {
        this.nameIsNotEmpty();
        this.nameHasLessThenTwentySymbols();
    }

    /**
     * nameIsNotEmpty checks if project name is not empty.
     */
    private nameIsNotEmpty(): void {
        if (this.name.length === 0) throw new Error('Project name can\'t be empty!');
    }

    /**
     * nameHasLessThenTwentySymbols checks if project name has less then 20 symbols.
     */
    private nameHasLessThenTwentySymbols(): void {
        if (this.name.length > MAX_NAME_LENGTH) throw new Error('Name should be less than 21 character!');
    }
}

/**
 * ProjectLimits is a type, used for describing project limits.
 */
export class ProjectLimits {
    public constructor(
        public bandwidthLimit: number = 0,
        public bandwidthUsed: number = 0,
        public storageLimit: number = 0,
        public storageUsed: number = 0,
        public objectCount: number = 0,
        public segmentCount: number = 0,
        public segmentLimit: number = 0,
        public segmentUsed: number = 0,
    ) {}
}

export class ProjectPage {
    public constructor(
        public projects: Project[] = [],
        public pageCount: number = 0,
        public currentPage: number = 0,
        public totalCount: number = 0,
    ) {}
}

/**
 * ProjectsPage class, used to describe paged projects list.
 */
export class ProjectsPage {
    public constructor(
        public projects: Project[] = [],
        public limit: number = 0,
        public offset: number = 0,
        public pageCount: number = 0,
        public currentPage: number = 0,
        public totalCount: number = 0,
    ) {}
}

/**
 * ProjectsPage class, used to describe paged projects list.
 */
export class ProjectsCursor {
    public constructor(
        public limit: number = DEFAULT_PAGE_LIMIT,
        public page: number = 1,
    ) {}
}

/**
 * DataStamp is storage/bandwidth usage stamp for satellite at some point in time
 */
export class DataStamp {
    public constructor(
        public value = 0,
        public intervalStart = new Date(),
    ) {}

    /**
     * Creates new empty instance of stamp with defined date
     * @param date - holds specific date of the date range
     * @returns Stamp - new empty instance of stamp with defined date
     */
    public static emptyWithDate(date: Date): DataStamp {
        return new DataStamp(0, date);
    }
}

/**
 * ProjectsStorageBandwidthDaily is used to describe project's daily storage and bandwidth usage.
 */
export class ProjectsStorageBandwidthDaily {
    public constructor(
        public storage: DataStamp[] = [],
        public allocatedBandwidth: DataStamp[] = [],
    ) {}
}

/**
 * ProjectInvitation represents a pending project member invitation.
 */
export class ProjectInvitation {
    public constructor(
        public projectID: string,
        public projectName: string,
        public projectDescription: string,
        public inviterEmail: string,
        public createdAt: Date,
    ) {}

    /**
     * Returns created date as a local string.
     */
    public get invitedDate(): string {
        const createdAt = new Date(this.createdAt);
        return createdAt.toLocaleString('en-US', { year: 'numeric', month: '2-digit', day: 'numeric' });
    }
}

/**
 * ProjectInvitationResponse represents a response to a project member invitation.
 */
export enum ProjectInvitationResponse {
    Decline,
    Accept,
}

/**
 * LimitRequestInfo holds data needed to request limit increase.
 */
export interface LimitRequestInfo {
    limitType: string
    currentLimit: string
    desiredLimit: string
}

/**
 * ProjectUsageDateRange is used to describe project's usage by date range.
 */
export interface ProjectUsageDateRange {
    since: Date;
    before: Date;
}

export enum LimitToChange {
    Storage = 'Storage',
    Bandwidth = 'Download',
}

export enum FieldToChange {
    Name = 'Name',
    Description = 'Description',
}

export enum LimitThreshold {
    Hundred = 'Hundred',
    Eighty = 'Eighty',
    CustomHundred = 'CustomHundred',
    CustomEighty = 'CustomEighty',
}

export enum LimitType {
    Storage = 'Storage',
    Egress = 'Egress',
    Segment = 'Segment',
}

export type LimitThresholdsReached = Record<LimitThreshold, LimitType[]>;

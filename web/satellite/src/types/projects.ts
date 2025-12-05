// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { ProjectRole } from '@/types/projectMembers';
import { Versioning } from '@/types/versioning';
import { DeleteProjectStep } from '@/types/accountActions';
import { PlacementDetails } from '@/types/buckets';

/**
 * Exposes all project-related functionality.
 */
export interface ProjectsApi {
    /**
     * Creates project.
     *
     * @param createProjectFields - contains project information
     * @param csrfProtectionToken - CSRF token
     * @throws Error
     */
    create(createProjectFields: ProjectFields, csrfProtectionToken: string): Promise<Project>;
    /**
     * Fetch projects.
     *
     * @returns Project[]
     * @throws Error
     */
    get(): Promise<Project[]>;

    /**
     * Delete project.
     *
     * @param projectId
     * @param step
     * @param data
     * @param csrfProtectionToken
     * @throws Error
     */
    delete(projectId: string, step: DeleteProjectStep, data: string, csrfProtectionToken: string): Promise<ProjectDeletionData | null>;

    /**
     * Fetch config for project.
     *
     * @param projectId - the project's ID
     * @returns ProjectConfig
     * @throws Error
     */
    getConfig(projectId: string): Promise<ProjectConfig>;

    /**
     * Update project name and description.
     *
     * @param projectId - project ID
     * @param updateProjectFields - project fields to update
     * @param csrfProtectionToken - CSRF token
     * @throws Error
     */
    update(projectId: string, updateProjectFields: UpdateProjectFields, csrfProtectionToken: string): Promise<void>;

    /**
     * Update project user specified limits.
     *
     * @param projectId - project ID
     * @param fields - project limits to update
     * @param csrfProtectionToken - CSRF token
     * @throws Error
     */
    updateLimits(projectId: string, fields: UpdateProjectLimitsFields, csrfProtectionToken: string): Promise<void>;

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
     * Get project emission impact
     *
     * @param projectID - project ID
     * @throws Error
     */
    getEmissionImpact(projectID: string): Promise<Emission>;

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
    getTotalUsageReportLink(start: number, end: number, includeCost: boolean, projectSummary: boolean, projectID: string): string

    /**
     * Get project daily usage by specific date range.
     *
     * @throws Error
     */
    getDailyUsage(projectID: string, start: Date, end: Date): Promise<ProjectsStorageBandwidthDaily>;

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
    respondToInvitation(projectID: string, response: ProjectInvitationResponse, csrfProtectionToken: string): Promise<void>;

    /**
     * Migrates project pricing from legacy to new pricing model.
     * @param projectID
     * @param csrfProtectionToken
     *
     * @throws Error
     */
    migratePricing(projectID: string, csrfProtectionToken: string): Promise<void>;
}

/**
 * MAX_NAME_LENGTH defines maximum amount of symbols for project name.
 */
export const MAX_NAME_LENGTH = 20;

/**
 * MAX_DESCRIPTION_LENGTH defines maximum amount of symbols for project description.
 */
export const MAX_DESCRIPTION_LENGTH = 100;

export enum ProjectEncryption {
    Automatic = 'Automatic',
    Manual = 'Manual',
}

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
        public memberCount: number = 0,
        public edgeURLOverrides?: EdgeURLOverrides,
        public versioning: Versioning = Versioning.NotSupported,
        public placement: number = 0,
        public storageUsed: number = 0,
        public bandwidthUsed: number = 0,
        public encryption: ProjectEncryption = ProjectEncryption.Manual,
        public isClassic: boolean = false,
    ) {}
}

/**
 * ProjectConfig is a type, used for project configuration.
 */
export class ProjectConfig {
    public constructor(
        public hasManagedPassphrase: boolean = false,
        public passphrase: string = '',
        public encryptPath: boolean = false,
        public isOwnerPaidTier: boolean = false,
        public hasPaidPrivileges: boolean = false,
        public _role: number = 1,
        public salt: string = '',
        public membersCount: number = 0,
        public availablePlacements: PlacementDetails[] = [],
        public computeAuthToken: string = '',
    ) {}

    public get role(): ProjectItemRole {
        switch (this._role) {
        case 1:
            return ProjectRole.Member;
        default:
            return ProjectRole.Admin;
        }
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
        public managePassphrase: boolean = false,
    ) {}
}

/**
 * ProjectLimits is a type, used for describing project limits.
 */
export class ProjectLimits {
    public constructor(
        public userSetBandwidthLimit: number | null = null,
        public userSetStorageLimit: number | null = null,
        public bandwidthLimit: number = 0,
        public bandwidthUsed: number = 0,
        public storageLimit: number = 0,
        public storageUsed: number = 0,
        public objectCount: number = 0,
        public segmentCount: number = 0,
        public segmentLimit: number = 0,
        public segmentUsed: number = 0,
        public bucketsLimit: number = 0,
        public bucketsUsed: number = 0,
    ) {}
}

export interface UpdateProjectFields {
    name: string;
    description: string;
}

export interface UpdateProjectLimitsFields {
    storageLimit?: string;
    bandwidthLimit?: string;
}

/**
 * ProjectDeletionData represents data returned by project deletion endpoint.
 */
export class ProjectDeletionData {
    public constructor(
        public lockEnabledBuckets: number,
        public buckets: number,
        public apiKeys: number,
        public currentUsage: boolean,
        public invoicingIncomplete: boolean,
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
        public settledBandwidth: DataStamp[] = [],
    ) {}
}

/**
 * Emission is used to describe project's emission impact done by different services.
 */
export class Emission {
    public constructor(
        public storjImpact: number = 0,
        public hyperscalerImpact: number = 0,
        public savedTrees: number = 0,
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

export type ManagePassphraseMode = 'auto' | 'manual';

/**
 * ProjectItemModel represents the view model for project items in the all projects dashboard.
 */
export class ProjectItemModel {
    public constructor(
        public id: string,
        public name: string,
        public description: string,
        public role: ProjectItemRole,
        public memberCount: number | null,
        public createdAt: Date,
        public storageUsed: number = 0,
        public bandwidthUsed: number = 0,
        public encryption: ProjectEncryption | undefined = undefined,
        public isClassic: boolean = false,
    ) {}
}

/**
 * ProjectItemRole represents the role of a user for a project item.
 */
export type ProjectItemRole = Exclude<ProjectRole, ProjectRole.InviteExpired>;

/**
 * PROJECT_ROLE_COLORS defines what colors project role tags should use.
 */
export const PROJECT_ROLE_COLORS: Record<ProjectRole, string> = {
    [ProjectRole.Admin]: 'purple',
    [ProjectRole.Member]: 'success',
    [ProjectRole.Owner]: 'primary',
    [ProjectRole.Invited]: 'warning',
    [ProjectRole.InviteExpired]: 'error',
};

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { Duration } from '@/utils/time';
import { ChangeEmailStep, DeleteAccountStep } from '@/types/accountActions';
import { SortDirection } from '@/types/common';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';

/**
 * Exposes all user-related functionality.
 */
export interface UsersApi {
    /**
     * Updates users full name and short name.
     *
     * @param user - contains information that should be updated
     * @throws Error
     */
    update(user: UpdatedUser): Promise<void>;

    /**
     * Fetch user.
     *
     * @returns User
     * @throws Error
     */
    get(): Promise<User>;

    /**
     * Fetches user frozen status.
     *
     * @returns boolean
     * @throws Error
     */
    getFrozenStatus(): Promise<FreezeStatus>;

    /**
     * Fetches user frozen status.
     *
     * @returns UserSettings
     * @throws Error
     */
    getUserSettings(): Promise<UserSettings>;

    /**
     * Used to fetch active user sessions.
     *
     * @throws Error
     */
    getSessions(cursor: SessionsCursor): Promise<SessionsPage>

    /**
     * Used to invalidate active user session by ID.
     *
     * @throws Error
     */
    invalidateSession(sessionID: string): Promise<void>

    /**
     * Changes user's settings.
     *
     * @param data
     * @returns UserSettings
     * @throws Error
     */
    updateSettings(data: SetUserSettingsData): Promise<UserSettings>;

    /**
     * Changes user's email.
     *
     * @param step
     * @param data
     * @throws Error
     */
    changeEmail(step: ChangeEmailStep, data: string): Promise<void>;

    /**
     * Marks user's account for deletion.
     *
     * @param step
     * @param data
     * @throws Error
     */
    deleteAccount(step: DeleteAccountStep, data: string): Promise<void>;

    /**
     * Enable user's MFA.
     *
     * @throws Error
     */
    enableUserMFA(passcode: string): Promise<void>;

    /**
     * Disable user's MFA.
     *
     * @throws Error
     */
    disableUserMFA(passcode: string, recoveryCode: string): Promise<void>;

    /**
     * Generate user's MFA secret.
     *
     * @throws Error
     */
    generateUserMFASecret(): Promise<string>;

    /**
     * Generate user's MFA recovery codes.
     *
     * @throws Error
     */
    generateUserMFARecoveryCodes(): Promise<string[]>;

    /**
     * Generate user's MFA recovery codes requiring a code.
     *
     * @throws Error
     */
    regenerateUserMFARecoveryCodes(passcode?: string, recoveryCode?: string): Promise<string[]>;

    /**
     * Request increase for user's project limit.
     *
     * @throws Error
     */
    requestProjectLimitIncrease(limit: string): Promise<void>;
}

/**
 * User class holds info for User entity.
 */
export class User {
    public constructor(
        public id: string = '',
        public fullName: string = '',
        public shortName: string = '',
        public email: string = '',
        public partner: string = '',
        public password: string = '',
        public projectLimit: number = 0,
        public projectStorageLimit: number = 0,
        public projectBandwidthLimit: number = 0,
        public projectSegmentLimit: number = 0,
        public paidTier: boolean = false,
        public isMFAEnabled: boolean = false,
        public isProfessional: boolean = false,
        public position: string = '',
        public companyName: string = '',
        public employeeCount: string = '',
        public haveSalesContact: boolean = false,
        public mfaRecoveryCodeCount: number = 0,
        public _createdAt: string | null = null,
        public pendingVerification: boolean = false,
        public trialExpiration: Date | null = null,
        public hasVarPartner: boolean = false,
        public signupPromoCode: string = '',
        public freezeStatus: FreezeStatus = new FreezeStatus(),
    ) { }

    public get createdAt(): Date | null {
        if (!this._createdAt) {
            return null;
        }
        const date = new Date(this._createdAt);
        if (date.toString().includes('Invalid')) {
            return null;
        }
        return date;
    }

    public getFullName(): string {
        return !this.shortName ? this.fullName : this.shortName;
    }

    public getExpirationInfo(daysBeforeNotify: number): ExpirationInfo {
        if (!this.trialExpiration) return { isCloseToExpiredTrial: false, days: 0 };

        const now = new Date();
        const diff = this.trialExpiration.getTime() - now.getTime();
        const millisecondsInDay = 24 * 60 * 60 * 1000;
        const daysBeforeNotifyInMilliseconds = daysBeforeNotify * millisecondsInDay;

        return {
            isCloseToExpiredTrial: diff < daysBeforeNotifyInMilliseconds,
            days: Math.round(Math.abs(diff) / millisecondsInDay),
        };
    }
}

export type ExpirationInfo = {
    isCloseToExpiredTrial: boolean;
    days: number;
}

/**
 * User class holds info for updating User.
 */
export class UpdatedUser {
    public constructor(
        public fullName: string = '',
        public shortName: string = '',
    ) { }

    public setFullName(value: string): void {
        this.fullName = value.trim();
    }

    public setShortName(value: string): void {
        this.shortName = value.trim();
    }

    public isValid(): boolean {
        return !!this.fullName;
    }
}

/**
 * Describes data used to set up user account.
 */
export interface AccountSetupData {
    isProfessional: boolean
    haveSalesContact: boolean
    interestedInPartnering: boolean
    firstName?: string
    lastName?: string
    fullName?: string
    position?: string
    companyName?: string
    employeeCount?: string
    storageNeeds?: string
    storageUseCase?: string
    otherUseCase?: string
    functionalArea?: string
}

/**
 * DisableMFARequest represents a request to disable multi-factor authentication.
 */
export class DisableMFARequest {
    public constructor(
        public passcode: string = '',
        public recoveryCode: string = '',
    ) { }
}

/**
 * TokenInfo represents an authentication token response.
 */
export class TokenInfo {
    public constructor(
        public token: string,
        public expiresAt: Date,
    ) { }
}

/**
 * UserSettings represents response from GET /auth/account/settings.
 */
export class UserSettings {
    public constructor(
        private _sessionDuration: number | null = null,
        public onboardingStart = false,
        public onboardingEnd = false,
        public passphrasePrompt = true,
        public onboardingStep: string | null = null,
        public noticeDismissal: NoticeDismissal = { fileGuide: false, serverSideEncryption: false, partnerUpgradeBanner: false, projectMembersPassphrase: false },
    ) { }

    public get sessionDuration(): Duration | null {
        if (this._sessionDuration) {
            return new Duration(this._sessionDuration);
        }
        return null;
    }
}

/**
 * User holds info for user session entity.
 */
export class Session {
    public constructor(
        public id: string = '',
        public userAgent: string = '',
        public expiresAt: Date = new Date(),
        public isCurrent: boolean = false,
    ) {}
}

/**
 * Holds user sessions sorting parameters.
 */
export enum SessionsOrderBy {
    userAgent = 1,
    expiresAt = 2,
}

/**
 * SessionsCursor is a type, used to describe paged user sessions pagination cursor.
 */
export class SessionsCursor {
    public constructor(
        public limit: number = DEFAULT_PAGE_LIMIT,
        public page: number = 1,
        public order: SessionsOrderBy = SessionsOrderBy.userAgent,
        public orderDirection: SortDirection = SortDirection.asc,
    ) {}
}

/**
 * ActiveSessionsPage is a type, used to describe paged user sessions list.
 */
export class SessionsPage {
    public constructor(
        public sessions: Session[] = [],
        public order: SessionsOrderBy = SessionsOrderBy.userAgent,
        public orderDirection: SortDirection = SortDirection.asc,
        public limit: number = 6,
        public pageCount: number = 0,
        public currentPage: number = 1,
        public totalCount: number = 0,
    ) {}
}

export interface NoticeDismissal {
    fileGuide: boolean
    serverSideEncryption: boolean
    partnerUpgradeBanner: boolean
    projectMembersPassphrase: boolean
    uploadOverwriteWarning?: boolean;
    versioningBetaBanner?: boolean;
}

export interface SetUserSettingsData {
    onboardingStart?: boolean
    onboardingEnd?: boolean;
    passphrasePrompt?: boolean;
    onboardingStep?: string | null;
    sessionDuration?: number;
    noticeDismissal?: NoticeDismissal;
}

/**
 * FreezeStatus represents a freeze-status endpoint response.
 */
export class FreezeStatus {
    public constructor(
        public frozen = false,
        public warned = false,
        public trialExpiredFrozen = false,
    ) { }
}

/**
 * OnboardingStep are the steps in the account setup dialog and onboarding stepper.
 */
export enum OnboardingStep {
    AccountTypeSelection = 'AccountTypeSelection',
    PersonalAccountForm = 'PersonalAccountForm',
    PlanTypeSelection = 'PlanTypeSelection',
    PricingPlanSelection = 'PricingPlanSelection',
    ManagedPassphraseOptIn = 'ManagedPassphraseOptIn',
    PricingPlan = 'PricingPlan',
    BusinessAccountForm = 'BusinessAccountForm',
    SetupComplete = 'SetupComplete',
    EncryptionPassphrase = 'EncryptionPassphrase',
    CreateBucket = 'CreateBucket',
    UploadFiles = 'UploadFiles',
    CreateAccess = 'CreateAccess',
}

export const ONBOARDING_STEPPER_STEPS = [
    OnboardingStep.EncryptionPassphrase,
    OnboardingStep.CreateBucket,
    OnboardingStep.UploadFiles,
    OnboardingStep.CreateAccess,
];

export const ACCOUNT_SETUP_STEPS = [
    OnboardingStep.AccountTypeSelection,
    OnboardingStep.PersonalAccountForm,
    OnboardingStep.ManagedPassphraseOptIn,
    OnboardingStep.PlanTypeSelection,
    OnboardingStep.PricingPlanSelection,
    OnboardingStep.PricingPlan,
    OnboardingStep.BusinessAccountForm,
    OnboardingStep.SetupComplete,
];
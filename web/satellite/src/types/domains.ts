// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { SortDirection } from '@/types/common';

/**
 * Exposes all domains-related functionality.
 */
export interface DomainsAPI {
    /**
     * Checks DNS records for provided domain.
     *
     * @throws Error
     */
    checkDNSRecords(domain: string, cname: string, txt: string[]): Promise<CheckDNSResponse>;

    /**
     * Registers domain on a server side.
     *
     * @throws Error
     */
    create(projectID: string, request: CreateDomainRequest, csrfProtectionToken: string): Promise<void>;

    /**
     * Removes domain from a server side.
     *
     * @throws Error
     */
    delete(projectID: string, subdomain: string, csrfProtectionToken: string): Promise<void>;

    /**
     * Returns paged domains list from a server side.
     *
     * @throws Error
     */
    getPaged(projectID: string, cursor: DomainsCursor): Promise<DomainsPage>;

    /**
     * Returns all domain names from a server side.
     *
     * @throws Error
     */
    getAllNames(projectID: string): Promise<string[]>;
}

export type CreateDomainRequest = {
    subdomain: string;
    accessID: string;
    prefix: string;
};

export class Domain {
    constructor(
        public name: string = '',
        public createdAt: Date = new Date(),
    ) { }
}

/**
 * Holds domains sorting parameters.
 */
export enum DomainsOrderBy {
    name = 1,
    createdAt = 2,
}

/**
 * DomainsCursor is a type, used to describe paged domains list.
 */
export class DomainsCursor {
    public constructor(
        public search: string = '',
        public limit: number = DEFAULT_PAGE_LIMIT,
        public page: number = 1,
        public order: DomainsOrderBy = DomainsOrderBy.name,
        public orderDirection: SortDirection = SortDirection.asc,
    ) {}
}

/**
 * DomainsPage is a type, used to describe paged domains list.
 */
export class DomainsPage {
    public constructor(
        public domains: Domain[] = [],
        public search: string = '',
        public order: DomainsOrderBy = DomainsOrderBy.name,
        public orderDirection: SortDirection = SortDirection.asc,
        public limit: number = 6,
        public pageCount: number = 0,
        public currentPage: number = 1,
        public totalCount: number = 0,
    ) {}
}

export enum NewDomainFlowStep {
    CustomDomain = 1,
    SetupDomainAccess,
    EnterNewPassphrase,
    PassphraseGenerated,
    SetupCNAME,
    SetupTXT,
    VerifyDomain,
    DomainConnected,
}

export type CheckDNSResponse = {
    isSuccess: boolean
    isVerifyError: boolean
    expectedCNAME: string
    expectedTXT: string[]
    gotCNAME: string
    gotTXT: string[]
};

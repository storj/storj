// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Exposes all domains-related functionality.
 */
export interface DomainsAPI {
    /**
     * Checks DNS records for provided domain.
     *
     * @throws Error
     */
    checkDNSRecords(domain: string, cname: string, txt: string[]): Promise<void>;
}

export class Domain {
    constructor(
        public id: string = '',
        public name: string = '',
        public createdAt: Date = new Date(),
    ) { }
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

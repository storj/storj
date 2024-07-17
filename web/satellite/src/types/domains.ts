// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

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

// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

export class Domain {
    constructor(
        public id: string = '',
        public name: number = 0,
        public createdAt: Date = new Date(),
    ) { }
}

export enum NewDomainFlowStep {
    CustomDomain,
    SetupCNAME,
    SetupTXT,
    VerifyDomain,
    DomainConnected,
}

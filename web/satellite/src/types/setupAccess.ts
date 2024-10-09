// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

export enum AccessType {
    APIKey = 'API-Key',
    S3 = 'S3-Credentials',
    AccessGrant = 'Access-Grant',
}

export enum PassphraseOption {
    UseExistingPassphrase = 'useExistingPassphrase',
    SetMyProjectPassphrase = 'setMyProjectPassphrase',
    GenerateNewPassphrase = 'generateNewPassphrase',
    EnterNewPassphrase = 'enterNewPassphrase',
}

export enum SetupStep {
    ChooseAccessStep = 'chooseAccess',
    EncryptionInfo = 'encryptionInfo',
    ChooseFlowStep = 'chooseFlow',
    AccessEncryption = 'accessEncryption',
    PassphraseGenerated = 'passphraseGenerated',
    EnterNewPassphrase = 'enterNewPassphrase',
    ChoosePermissionsStep = 'choosePermission',
    SelectBucketsStep = 'selectBuckets',
    OptionalExpirationStep = 'optionalExpiration',
    ConfirmDetailsStep = 'confirmDetails',
    AccessCreatedStep = 'accessCreated',
}

export enum FlowType {
    FullAccess = 'fullAccess',
    Advanced = 'advanced',
}

export enum Permission {
    All = 'all',
    Read = 'Read',
    Write = 'Write',
    List = 'List',
    Delete = 'Delete',
}

export enum BucketsOption {
    All = 'all',
    Select = 'select',
}

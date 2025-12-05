// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export enum ManageProjectPassphraseStep {
    ManageOptions = 1,
    Create,
    EncryptionPassphrase,
    PassphraseGenerated,
    EnterPassphrase,
    Success,
    Switch,
    Clear,
}

export enum PassphraseOption {
    GeneratePassphrase,
    EnterPassphrase,
}

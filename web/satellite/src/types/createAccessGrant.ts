// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import CreateNewAccessIcon from '@/../static/images/accessGrants/newCreateFlow/createNewAccess.svg';
import ChoosePermissionIcon from '@/../static/images/accessGrants/newCreateFlow/choosePermission.svg';
import AccessEncryptionIcon from '@/../static/images/accessGrants/newCreateFlow/accessEncryption.svg';
import PassphraseGeneratedIcon from '@/../static/images/accessGrants/newCreateFlow/passphraseGenerated.svg';
import AccessCreatedIcon from '@/../static/images/accessGrants/newCreateFlow/accessCreated.svg';
import CLIAccessCreatedIcon from '@/../static/images/accessGrants/newCreateFlow/cliAccessCreated.svg';
import CredentialsCreatedIcon from '@/../static/images/accessGrants/newCreateFlow/credentialsCreated.svg';
import EncryptionInfoIcon from '@/../static/images/accessGrants/newCreateFlow/encryptionInfo.svg';
import ConfirmDetailsIcon from '@/../static/images/accessGrants/newCreateFlow/confirmDetails.svg';

export interface IconAndTitle {
    icon: string;
    title: string;
}

export enum AccessType {
    APIKey = 'API Key',
    S3 = 'S3 Credentials',
    AccessGrant = 'Access Grant',
}

export enum PassphraseOption {
    UseExistingPassphrase = 'useExistingPassphrase',
    SetMyProjectPassphrase = 'setMyProjectPassphrase',
    GenerateNewPassphrase = 'generateNewPassphrase',
    EnterNewPassphrase = 'enterNewPassphrase',
}

export enum CreateAccessStep {
    CreateNewAccess = 'createNewAccess',
    ChoosePermission = 'choosePermission',
    EncryptionInfo = 'encryptionInfo',
    AccessEncryption = 'accessEncryption',
    PassphraseGenerated = 'passphraseGenerated',
    EnterMyPassphrase = 'enterMyPassphrase',
    EnterNewPassphrase = 'enterNewPassphrase',
    ConfirmDetails = 'confirmDetails',
    AccessCreated = 'accessCreated',
    CLIAccessCreated = 'cliAccessCreated',
    CredentialsCreated = 'credentialsCreated',
}

export enum Permission {
    All = 'all',
    Read = 'Read',
    Write = 'Write',
    List = 'List',
    Delete = 'Delete',
}

export const STEP_ICON_AND_TITLE: Record<CreateAccessStep, IconAndTitle> = {
    [CreateAccessStep.CreateNewAccess]: {
        icon: CreateNewAccessIcon,
        title: 'Create a new access',
    },
    [CreateAccessStep.ChoosePermission]: {
        icon: ChoosePermissionIcon,
        title: 'Choose permissions',
    },
    [CreateAccessStep.EncryptionInfo]: {
        icon: EncryptionInfoIcon,
        title: 'Encryption information',
    },
    [CreateAccessStep.AccessEncryption]: {
        icon: AccessEncryptionIcon,
        title: 'Access encryption',
    },
    [CreateAccessStep.PassphraseGenerated]: {
        icon: PassphraseGeneratedIcon,
        title: 'Passphrase generated',
    },
    [CreateAccessStep.EnterMyPassphrase]: {
        icon: AccessEncryptionIcon,
        title: 'Enter my passphrase',
    },
    [CreateAccessStep.EnterNewPassphrase]: {
        icon: AccessEncryptionIcon,
        title: 'Enter a new passphrase',
    },
    [CreateAccessStep.ConfirmDetails]: {
        icon: ConfirmDetailsIcon,
        title: 'Confirm details',
    },
    [CreateAccessStep.AccessCreated]: {
        icon: AccessCreatedIcon,
        title: 'Access created',
    },
    [CreateAccessStep.CredentialsCreated]: {
        icon: CredentialsCreatedIcon,
        title: 'Credentials created',
    },
    [CreateAccessStep.CLIAccessCreated]: {
        icon: CLIAccessCreatedIcon,
        title: 'CLI access created',
    },
};

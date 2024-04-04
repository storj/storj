// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import CreateNewAccessIcon from '@/assets/createAccessGrantFlow/createNewAccess.svg';
import ChoosePermissionIcon from '@/assets/createAccessGrantFlow/choosePermission.svg';
import AccessEncryptionIcon from '@/assets/createAccessGrantFlow/accessEncryption.svg';
import PassphraseGeneratedIcon from '@/assets/createAccessGrantFlow/passphraseGenerated.svg';
import AccessCreatedIcon from '@/assets/createAccessGrantFlow/accessCreated.svg';
import CLIAccessCreatedIcon from '@/assets/createAccessGrantFlow/cliAccessCreated.svg';
import CredentialsCreatedIcon from '@/assets/createAccessGrantFlow/credentialsCreated.svg';
import EncryptionInfoIcon from '@/assets/createAccessGrantFlow/encryptionInfo.svg';
import ConfirmDetailsIcon from '@/assets/createAccessGrantFlow/confirmDetails.svg';

export interface IconAndTitle {
    icon: string;
    title: string;
}

export interface Exposed {
    setName: (newName: string) => void,
    setTypes: (newTypes: AccessType[]) => void,
}

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

export interface AccessGrantEndDate {
    title: string;
    date: Date | null;
}

export const ACCESS_TYPE_LINKS: Record<AccessType, string> = {
    [AccessType.AccessGrant]: 'https://docs.storj.io/dcs/concepts/access/access-grants',
    [AccessType.S3]: 'https://docs.storj.io/dcs/api-reference/s3-compatible-gateway',
    [AccessType.APIKey]: 'https://docs.storj.io/dcs/getting-started/quickstart-uplink-cli/generate-access-grants-and-tokens/generate-a-token',
};

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
        title: 'API key created',
    },
};

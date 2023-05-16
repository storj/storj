// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { Component } from 'vue';

import CreateNewAccessIcon from '@/../static/images/accessGrants/newCreateFlow/createNewAccess.svg';
import ChoosePermissionIcon from '@/../static/images/accessGrants/newCreateFlow/choosePermission.svg';
import AccessEncryptionIcon from '@/../static/images/accessGrants/newCreateFlow/accessEncryption.svg';
import PassphraseGeneratedIcon from '@/../static/images/accessGrants/newCreateFlow/passphraseGenerated.svg';
import AccessCreatedIcon from '@/../static/images/accessGrants/newCreateFlow/accessCreated.svg';
import CLIAccessCreatedIcon from '@/../static/images/accessGrants/newCreateFlow/cliAccessCreated.svg';
import CredentialsCreatedIcon from '@/../static/images/accessGrants/newCreateFlow/credentialsCreated.svg';
import EncryptionInfoIcon from '@/../static/images/accessGrants/newCreateFlow/encryptionInfo.svg';
import ConfirmDetailsIcon from '@/../static/images/accessGrants/newCreateFlow/confirmDetails.svg';
import TypeIcon from '@/../static/images/accessGrants/newCreateFlow/typeIcon.svg';
import NameIcon from '@/../static/images/accessGrants/newCreateFlow/nameIcon.svg';
import PermissionsIcon from '@/../static/images/accessGrants/newCreateFlow/permissionsIcon.svg';
import BucketsIcon from '@/../static/images/accessGrants/newCreateFlow/bucketsIcon.svg';
import EndDateIcon from '@/../static/images/accessGrants/newCreateFlow/endDateIcon.svg';
import EncryptionPassphraseIcon from '@/../static/images/accessGrants/newCreateFlow/encryptionPassphraseIcon.svg';

export interface IconAndTitle {
    icon: Component;
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

export enum FunctionalContainer {
    Type = 'type',
    Name = 'name',
    Permissions = 'permissions',
    Buckets = 'buckets',
    EndDate = 'endDate',
    EncryptionPassphrase = 'encryptionPassphrase',
}

export const FUNCTIONAL_CONTAINER_ICON_AND_TITLE: Record<FunctionalContainer, IconAndTitle> = {
    [FunctionalContainer.Type]: {
        icon: TypeIcon,
        title: 'Type',
    },
    [FunctionalContainer.Name]: {
        icon: NameIcon,
        title: 'Name',
    },
    [FunctionalContainer.Permissions]: {
        icon: PermissionsIcon,
        title: 'Permissions',
    },
    [FunctionalContainer.Buckets]: {
        icon: BucketsIcon,
        title: 'Buckets',
    },
    [FunctionalContainer.EndDate]: {
        icon: EndDateIcon,
        title: 'End Date',
    },
    [FunctionalContainer.EncryptionPassphrase]: {
        icon: EncryptionPassphraseIcon,
        title: 'Encryption Passphrase',
    },
};

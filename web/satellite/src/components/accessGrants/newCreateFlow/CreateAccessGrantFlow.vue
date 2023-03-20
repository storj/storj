// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <div class="modal__header">
                    <component :is="STEP_ICON_AND_TITLE[step].icon" />
                    <h1 class="modal__header__title">{{ STEP_ICON_AND_TITLE[step].title }}</h1>
                </div>
                <CreateNewAccessStep
                    v-if="step === CreateAccessStep.CreateNewAccess"
                    :on-select-type="selectAccessType"
                    :selected-access-types="selectedAccessTypes"
                    :name="accessName"
                    :set-name="setAccessName"
                    :on-continue="setSecondStepBasedOnAccessType"
                />
                <EncryptionInfoStep
                    v-if="step === CreateAccessStep.EncryptionInfo"
                    :on-back="() => setStep(CreateAccessStep.CreateNewAccess)"
                    :on-continue="() => setStep(CreateAccessStep.ChoosePermission)"
                />
                <ChoosePermissionStep
                    v-if="step === CreateAccessStep.ChoosePermission"
                    :on-select-permission="selectPermissions"
                    :selected-permissions="selectedPermissions"
                    :on-back="setFirstStepBasedOnAccessType"
                    :on-continue="() => setStep(
                        selectedAccessTypes.includes(AccessType.APIKey) ? CreateAccessStep.ConfirmDetails : CreateAccessStep.AccessEncryption
                    )"
                    :selected-buckets="selectedBuckets"
                    :on-select-bucket="selectBucket"
                    :on-select-all-buckets="selectAllBuckets"
                    :on-unselect-bucket="unselectBucket"
                    :not-after="notAfter"
                    :on-set-not-after="setNotAfter"
                    :not-after-label="notAfterLabel"
                    :on-set-not-after-label="setNotAfterLabel"
                />
                <AccessEncryptionStep
                    v-if="step === CreateAccessStep.AccessEncryption"
                    :on-back="() => setStep(CreateAccessStep.ChoosePermission)"
                    :on-continue="setStepBasedOnPassphraseOption"
                    :passphrase-option="passphraseOption"
                    :set-option="setPassphraseOption"
                />
                <EnterPassphraseStep
                    v-if="step === CreateAccessStep.EnterMyPassphrase"
                    :is-new-passphrase="false"
                    :on-back="() => setStep(CreateAccessStep.AccessEncryption)"
                    :on-continue="() => setStep(CreateAccessStep.ConfirmDetails)"
                    :passphrase="enteredPassphrase"
                    :set-passphrase="setPassphrase"
                    info="Enter the encryption passphrase used for this project to create this access grant."
                />
                <EnterPassphraseStep
                    v-if="step === CreateAccessStep.EnterNewPassphrase"
                    :is-new-passphrase="true"
                    :on-back="() => setStep(CreateAccessStep.AccessEncryption)"
                    :on-continue="() => setStep(CreateAccessStep.ConfirmDetails)"
                    :passphrase="enteredPassphrase"
                    :set-passphrase="setPassphrase"
                    info="This passphrase will be used to encrypt all the files you upload using this access grant.
                        You will need it to access these files in the future."
                />
                <PassphraseGeneratedStep
                    v-if="step === CreateAccessStep.PassphraseGenerated"
                    :on-back="() => setStep(CreateAccessStep.AccessEncryption)"
                    :on-continue="() => setStep(CreateAccessStep.ConfirmDetails)"
                    :passphrase="generatedPassphrase"
                    :name="accessName"
                />
                <ConfirmDetailsStep
                    v-if="step === CreateAccessStep.ConfirmDetails"
                    :access-types="selectedAccessTypes"
                    :is-loading="isLoading"
                    :not-after-label="notAfterLabel"
                    :selected-buckets="selectedBuckets"
                    :name="accessName"
                    :selected-permissions="selectedPermissions"
                    :on-back="setPreviousStepFromConfirm"
                    :on-continue="setLastStep"
                />
                <AccessCreatedStep
                    v-if="step === CreateAccessStep.AccessCreated"
                    :on-continue="() => setStep(CreateAccessStep.CredentialsCreated)"
                    :access-grant="accessGrant"
                    :name="accessName"
                    :access-types="selectedAccessTypes"
                />
                <CLIAccessCreatedStep
                    v-if="step === CreateAccessStep.CLIAccessCreated"
                    :api-key="cliAccess"
                    :name="accessName"
                />
                <S3CredentialsCreatedStep
                    v-if="step === CreateAccessStep.CredentialsCreated"
                    :credentials="edgeCredentials"
                    :name="accessName"
                />
                <div v-if="isLoading" class="modal__blur" />
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { generateMnemonic } from 'bip39';

import { useNotify, useRoute, useRouter, useStore } from '@/utils/hooks';
import { RouteConfig } from '@/router';
import {
    AccessType,
    CreateAccessStep,
    PassphraseOption,
    Permission,
    STEP_ICON_AND_TITLE,
} from '@/types/createAccessGrant';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { LocalData } from '@/utils/localData';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { MetaUtils } from '@/utils/meta';
import { AnalyticsHttpApi } from '@/api/analytics';
import { OBJECTS_MUTATIONS } from '@/store/modules/objects';

import VModal from '@/components/common/VModal.vue';
import CreateNewAccessStep from '@/components/accessGrants/newCreateFlow/steps/CreateNewAccessStep.vue';
import ChoosePermissionStep from '@/components/accessGrants/newCreateFlow/steps/ChoosePermissionStep.vue';
import AccessEncryptionStep from '@/components/accessGrants/newCreateFlow/steps/AccessEncryptionStep.vue';
import EnterPassphraseStep from '@/components/accessGrants/newCreateFlow/steps/EnterPassphraseStep.vue';
import PassphraseGeneratedStep from '@/components/accessGrants/newCreateFlow/steps/PassphraseGeneratedStep.vue';
import EncryptionInfoStep from '@/components/accessGrants/newCreateFlow/steps/EncryptionInfoStep.vue';
import AccessCreatedStep from '@/components/accessGrants/newCreateFlow/steps/AccessCreatedStep.vue';
import CLIAccessCreatedStep from '@/components/accessGrants/newCreateFlow/steps/CLIAccessCreatedStep.vue';
import S3CredentialsCreatedStep from '@/components/accessGrants/newCreateFlow/steps/S3CredentialsCreatedStep.vue';
import ConfirmDetailsStep from '@/components/accessGrants/newCreateFlow/steps/ConfirmDetailsStep.vue';

const router = useRouter();
const route = useRoute();
const notify = useNotify();
const store = useStore();

const initPermissions = [
    Permission.Read,
    Permission.Write,
    Permission.Delete,
    Permission.List,
];

/**
 * Indicates if user has to be prompt to enter project passphrase.
 */
const isPromptForPassphrase = computed((): boolean => {
    return store.state.objectsModule.promptForPassphrase;
});

/**
 * Returns passphrase from store.
 */
const storedPassphrase = computed((): string => {
    return store.state.objectsModule.passphrase;
});

const worker = ref<Worker>();
const isLoading = ref<boolean>(false);
const step = ref<CreateAccessStep>(CreateAccessStep.CreateNewAccess);
const selectedAccessTypes = ref<AccessType[]>([]);
const selectedPermissions = ref<Permission[]>(initPermissions);
const selectedBuckets = ref<string[]>([]);
const passphraseOption = ref<PassphraseOption>(
    isPromptForPassphrase.value ? PassphraseOption.SetMyProjectPassphrase : PassphraseOption.UseExistingPassphrase,
);
const enteredPassphrase = ref<string>('');
const generatedPassphrase = ref<string>('');
const accessName = ref<string>('');
const notAfter = ref<Date | undefined>(undefined);
const notAfterLabel = ref<string>('No end date');

// Generated values.
const cliAccess = ref<string>('');
const accessGrant = ref<string>('');
const edgeCredentials = ref<EdgeCredentials>(new EdgeCredentials());

const FIRST_PAGE = 1;
const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

/**
 * Selects access type.
 */
function selectAccessType(type: AccessType) {
    // "access grant" and "s3 credentials" can be selected at the same time,
    // but "API key" cannot be selected if either "access grant" or "s3 credentials" is selected.
    switch (type) {
    case AccessType.AccessGrant:
        // Unselect API key if was selected.
        unselectAPIKeyAccessType();

        // Unselect Access grant if was selected.
        if (selectedAccessTypes.value.includes(AccessType.AccessGrant)) {
            selectedAccessTypes.value = selectedAccessTypes.value.filter(t => t !== AccessType.AccessGrant);
            return;
        }

        // Select Access grant.
        selectedAccessTypes.value.push(type);
        break;
    case AccessType.S3:
        // Unselect API key if was selected.
        unselectAPIKeyAccessType();

        // Unselect S3 if was selected.
        if (selectedAccessTypes.value.includes(AccessType.S3)) {
            selectedAccessTypes.value = selectedAccessTypes.value.filter(t => t !== AccessType.S3);
            return;
        }

        // Select S3.
        selectedAccessTypes.value.push(type);
        break;
    case AccessType.APIKey:
        // Unselect Access grant and S3 if were selected.
        if (selectedAccessTypes.value.includes(AccessType.AccessGrant) || selectedAccessTypes.value.includes(AccessType.S3)) {
            selectedAccessTypes.value = selectedAccessTypes.value.filter(t => t === AccessType.APIKey);
        }

        // Unselect API key if was selected.
        if (selectedAccessTypes.value.includes(AccessType.APIKey)) {
            selectedAccessTypes.value = selectedAccessTypes.value.filter(t => t !== AccessType.APIKey);
            return;
        }

        // Select API key.
        selectedAccessTypes.value.push(type);
    }
}

/**
 * Sets passphrase option.
 */
function setPassphraseOption(option: PassphraseOption): void {
    passphraseOption.value = option;
}

/**
 * Sets entered passphrase.
 */
function setPassphrase(value: string): void {
    enteredPassphrase.value = value;
}

/**
 * Sets not after (end date) caveat.
 */
function setNotAfter(date: Date | undefined): void {
    notAfter.value = date;
}

/**
 * Sets previous step from confirm step.
 */
function setPreviousStepFromConfirm(): void {
    switch (true) {
    case selectedAccessTypes.value.includes(AccessType.APIKey):
        step.value = CreateAccessStep.ChoosePermission;
        break;
    case passphraseOption.value === PassphraseOption.SetMyProjectPassphrase:
        step.value = CreateAccessStep.EnterMyPassphrase;
        break;
    case passphraseOption.value === PassphraseOption.UseExistingPassphrase:
        step.value = CreateAccessStep.AccessEncryption;
        break;
    case passphraseOption.value === PassphraseOption.EnterNewPassphrase:
        step.value = CreateAccessStep.EnterNewPassphrase;
        break;
    case passphraseOption.value === PassphraseOption.GenerateNewPassphrase:
        step.value = CreateAccessStep.PassphraseGenerated;
    }
}

/**
 * Sets not after (end date) label.
 */
function setNotAfterLabel(label: string): void {
    notAfterLabel.value = label;
}

/**
 * Unselects API key access type.
 */
function unselectAPIKeyAccessType(): void {
    if (selectedAccessTypes.value.includes(AccessType.APIKey)) {
        selectedAccessTypes.value = selectedAccessTypes.value.filter(t => t !== AccessType.APIKey);
    }
}

/**
 * Selects access grant permissions.
 */
function selectPermissions(permission: Permission): void {
    switch (permission) {
    case Permission.All:
        if (selectedPermissions.value.length === 4) {
            selectedPermissions.value = [];
            return;
        }

        selectedPermissions.value = initPermissions;
        break;
    case Permission.Delete:
        handlePermissionSelection(Permission.Delete);
        break;
    case Permission.List:
        handlePermissionSelection(Permission.List);
        break;
    case Permission.Write:
        handlePermissionSelection(Permission.Write);
        break;
    case Permission.Read:
        handlePermissionSelection(Permission.Read);
    }
}

/**
 * Handles permission select/unselect.
 */
function handlePermissionSelection(permission: Permission) {
    if (selectedPermissions.value.includes(permission)) {
        selectedPermissions.value = selectedPermissions.value.filter(p => p !== permission);
        return;
    }

    selectedPermissions.value.push(permission);
}

/**
 * Clears bucket selection which means grant access to all buckets.
 */
function selectAllBuckets() {
    selectedBuckets.value = [];
}

/**
 * Select some specific bucket.
 */
function selectBucket(bucket: string) {
    selectedBuckets.value.push(bucket);
}

/**
 * Unselect some specific bucket.
 */
function unselectBucket(bucket: string) {
    selectedBuckets.value = selectedBuckets.value.filter(b => b !== bucket);
}

/**
 * Sets access grant name from input field.
 * @param value
 */
function setAccessName(value: string): void {
    accessName.value = value;
}

/**
 * Sets current step.
 */
function setStep(stepArg: CreateAccessStep): void {
    step.value = stepArg;
}

/**
 * Sets second step based on selected access type.
 * If access types include 'S3' and local storage value is false we set 'Encryption info step'.
 * If not then we set regular second step (Permissions).
 */
function setSecondStepBasedOnAccessType(): void {
    // Unfortunately local storage updates are not reactive so putting it inside computed property doesn't do anything.
    // That's why we explicitly call it here.
    const shouldShowInfo = !LocalData.getServerSideEncryptionModalHidden() && selectedAccessTypes.value.includes(AccessType.S3);
    if (shouldShowInfo) {
        setStep(CreateAccessStep.EncryptionInfo);
        return;
    }

    setStep(CreateAccessStep.ChoosePermission);
}

/**
 * Sets first step based on selected access type.
 * If access types include 'S3' and local storage value is false we set 'Encryption info step'.
 * If not then we set regular first step (Create access).
 */
function setFirstStepBasedOnAccessType(): void {
    // Unfortunately local storage updates are not reactive so putting it inside computed property doesn't do anything.
    // That's why we explicitly call it here.
    const shouldShowInfo = !LocalData.getServerSideEncryptionModalHidden() && selectedAccessTypes.value.includes(AccessType.S3);
    if (shouldShowInfo) {
        setStep(CreateAccessStep.EncryptionInfo);
        return;
    }

    setStep(CreateAccessStep.CreateNewAccess);
}

/**
 * Sets next step depending on selected passphrase option.
 */
function setStepBasedOnPassphraseOption(): void {
    switch (passphraseOption.value) {
    case PassphraseOption.SetMyProjectPassphrase:
        step.value = CreateAccessStep.EnterMyPassphrase;
        break;
    case PassphraseOption.EnterNewPassphrase:
        step.value = CreateAccessStep.EnterNewPassphrase;
        break;
    case PassphraseOption.GenerateNewPassphrase:
        step.value = CreateAccessStep.PassphraseGenerated;
        break;
    case PassphraseOption.UseExistingPassphrase:
        step.value = CreateAccessStep.ConfirmDetails;
    }
}

/**
 * Closes create access grant flow.
 */
function closeModal(): void {
    router.push(RouteConfig.AccessGrants.path);
}

/**
 * Sets local worker with worker instantiated in store.
 * Also sets worker's onmessage and onerror logic.
 */
function setWorker(): void {
    worker.value = store.state.accessGrantsModule.accessGrantsWebWorker;
    if (worker.value) {
        worker.value.onerror = (error: ErrorEvent) => {
            notify.error(error.message, AnalyticsErrorEventSource.CREATE_AG_MODAL);
        };
    }
}

/**
 * Generates CLI access.
 */
async function createCLIAccess(): Promise<void> {
    if (!worker.value) {
        throw new Error('Web worker is not initialized.');
    }

    // creates fresh new API key.
    const cleanAPIKey: AccessGrant = await store.dispatch(ACCESS_GRANTS_ACTIONS.CREATE, accessName.value);

    try {
        await store.dispatch(ACCESS_GRANTS_ACTIONS.FETCH, FIRST_PAGE);
    } catch (error) {
        await notify.error(`Unable to fetch Access Grants. ${error.message}`, AnalyticsErrorEventSource.CREATE_AG_MODAL);
    }

    let permissionsMsg = {
        'type': 'SetPermission',
        'buckets': selectedBuckets.value,
        'apiKey': cleanAPIKey.secret,
        'isDownload': selectedPermissions.value.includes(Permission.Read),
        'isUpload': selectedPermissions.value.includes(Permission.Write),
        'isList': selectedPermissions.value.includes(Permission.List),
        'isDelete': selectedPermissions.value.includes(Permission.Delete),
        'notBefore': new Date().toISOString(),
    };

    if (notAfter.value) permissionsMsg = Object.assign(permissionsMsg, { 'notAfter': notAfter.value.toISOString() });

    await worker.value.postMessage(permissionsMsg);

    const grantEvent: MessageEvent = await new Promise(resolve => {
        if (worker.value) {
            worker.value.onmessage = resolve;
        }
    });
    if (grantEvent.data.error) {
        throw new Error(grantEvent.data.error);
    }

    cliAccess.value = grantEvent.data.value;

    if (selectedAccessTypes.value.includes(AccessType.APIKey)) {
        analytics.eventTriggered(AnalyticsEvent.API_ACCESS_CREATED);
    }
}

/**
 * Generates access grant.
 */
async function createAccessGrant(): Promise<void> {
    if (!worker.value) {
        throw new Error('Web worker is not initialized.');
    }

    // creates access credentials.
    const satelliteNodeURL = MetaUtils.getMetaContent('satellite-nodeurl');

    const salt = await store.dispatch(PROJECTS_ACTIONS.GET_SALT, store.getters.selectedProject.id);

    let usedPassphrase = '';
    switch (passphraseOption.value) {
    case PassphraseOption.UseExistingPassphrase:
        usedPassphrase = storedPassphrase.value;
        break;
    case PassphraseOption.EnterNewPassphrase:
    case PassphraseOption.SetMyProjectPassphrase:
        usedPassphrase = enteredPassphrase.value;
        break;
    case PassphraseOption.GenerateNewPassphrase:
        usedPassphrase = generatedPassphrase.value;
    }

    if (!usedPassphrase) {
        throw new Error('Passphrase can\'t be empty');
    }

    worker.value.postMessage({
        'type': 'GenerateAccess',
        'apiKey': cliAccess.value,
        'passphrase': usedPassphrase,
        'salt': salt,
        'satelliteNodeURL': satelliteNodeURL,
    });

    const accessEvent: MessageEvent = await new Promise(resolve => {
        if (worker.value) {
            worker.value.onmessage = resolve;
        }
    });
    if (accessEvent.data.error) {
        throw new Error(accessEvent.data.error);
    }

    accessGrant.value = accessEvent.data.value;

    if (selectedAccessTypes.value.includes(AccessType.AccessGrant)) {
        analytics.eventTriggered(AnalyticsEvent.ACCESS_GRANT_CREATED);
    }
}

/**
 * Generates edge credentials.
 */
async function createEdgeCredentials(): Promise<void> {
    edgeCredentials.value = await store.dispatch(
        ACCESS_GRANTS_ACTIONS.GET_GATEWAY_CREDENTIALS, { accessGrant: accessGrant.value },
    );
    analytics.eventTriggered(AnalyticsEvent.GATEWAY_CREDENTIALS_CREATED);
}

/**
 * Generates access and sets the last step depending on selected access type.
 */
async function setLastStep(): Promise<void> {
    if (isLoading.value) {
        return;
    }

    isLoading.value = true;

    try {
        switch (true) {
        case selectedAccessTypes.value.includes(AccessType.APIKey):
            await createCLIAccess();

            step.value = CreateAccessStep.CLIAccessCreated;
            break;
        case selectedAccessTypes.value.includes(AccessType.AccessGrant) && selectedAccessTypes.value.includes(AccessType.S3):
            await createCLIAccess();
            await createAccessGrant();
            await createEdgeCredentials();

            step.value = CreateAccessStep.AccessCreated;
            break;
        case selectedAccessTypes.value.includes(AccessType.S3):
            await createCLIAccess();
            await createAccessGrant();
            await createEdgeCredentials();

            step.value = CreateAccessStep.CredentialsCreated;
            break;
        case selectedAccessTypes.value.includes(AccessType.AccessGrant):
            await createCLIAccess();
            await createAccessGrant();

            step.value = CreateAccessStep.AccessCreated;
        }

        // This is an action to handle case if user sets project level passphrase.
        if (
            passphraseOption.value === PassphraseOption.SetMyProjectPassphrase &&
            !selectedAccessTypes.value.includes(AccessType.APIKey)
        ) {
            store.commit(OBJECTS_MUTATIONS.SET_GATEWAY_CREDENTIALS, new EdgeCredentials());
            store.commit(OBJECTS_MUTATIONS.SET_PASSPHRASE, enteredPassphrase.value);
            store.commit(OBJECTS_MUTATIONS.SET_PROMPT_FOR_PASSPHRASE, false);
        }
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.CREATE_AG_MODAL);
    }

    isLoading.value = false;
}

onMounted(async () => {
    if (route.params?.accessType) {
        selectedAccessTypes.value.push(route.params?.accessType as AccessType);
    }

    setWorker();
    generatedPassphrase.value = generateMnemonic();

    try {
        await store.dispatch(BUCKET_ACTIONS.FETCH_ALL_BUCKET_NAMES);
    } catch (error) {
        notify.error(`Unable to fetch all bucket names. ${error.message}`, AnalyticsErrorEventSource.CREATE_AG_MODAL);
    }
});
</script>

<style scoped lang="scss">
.modal {
    width: 346px;
    padding: 32px;
    display: flex;
    flex-direction: column;
    position: relative;

    @media screen and (max-width: 460px) {
        width: 280px;
        padding: 16px;
    }

    &__header {
        display: flex;
        align-items: center;
        padding-bottom: 16px;
        border-bottom: 1px solid var(--c-grey-2);

        @media screen and (max-width: 460px) {
            flex-direction: column;
            align-items: flex-start;
        }

        &__title {
            margin-left: 16px;
            font-family: 'font_bold', sans-serif;
            font-size: 24px;
            line-height: 31px;
            letter-spacing: -0.02em;
            color: var(--c-black);
            text-align: left;

            @media screen and (max-width: 460px) {
                margin: 10px 0 0;
            }
        }
    }

    &__blur {
        position: absolute;
        left: 0;
        right: 0;
        bottom: 0;
        top: 0;
        background-color: rgb(0 0 0 / 10%);
        border-radius: 10px;
    }
}
</style>

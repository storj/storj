// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        min-width="320px"
        max-width="420px"
        transition="fade-transition"
        scrollable
        :persistent="isCreating"
    >
        <v-card ref="innerContent" rounded="xlg">
            <v-card-item class="pa-5 pl-7 pos-relative">
                <template #prepend>
                    <img class="d-block" :src="STEP_ICON_AND_TITLE[step].icon" alt="icon">
                </template>

                <v-card-title class="font-weight-bold">
                    {{ stepInfos[step].ref.value?.title }}
                </v-card-title>

                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isCreating"
                        @click="model = false"
                    />
                </template>

                <v-progress-linear height="2px" indeterminate absolute location="bottom" :active="isFetching || isCreating" />
            </v-card-item>

            <v-divider />

            <v-window
                v-model="step"
                class="overflow-y-auto create-access-dialog__window"
                :class="{ 'create-access-dialog__window--loading': isFetching }"
            >
                <v-window-item :value="CreateAccessStep.CreateNewAccess">
                    <create-new-access-step
                        :ref="stepInfos[CreateAccessStep.CreateNewAccess].ref"
                        @name-changed="newName => name = newName"
                        @types-changed="newTypes => accessTypes = newTypes"
                    />
                </v-window-item>

                <v-window-item :value="CreateAccessStep.EncryptionInfo">
                    <encryption-info-step :ref="stepInfos[CreateAccessStep.EncryptionInfo].ref" />
                </v-window-item>

                <v-window-item :value="CreateAccessStep.ChoosePermission">
                    <choose-permissions-step
                        :ref="stepInfos[CreateAccessStep.ChoosePermission].ref"
                        @buckets-changed="newBuckets => buckets = newBuckets"
                        @permissions-changed="newPerms => permissions = newPerms"
                        @end-date-changed="newDate => endDate = newDate"
                    />
                </v-window-item>

                <v-window-item :value="CreateAccessStep.AccessEncryption">
                    <access-encryption-step
                        :ref="stepInfos[CreateAccessStep.AccessEncryption].ref"
                        @select-option="newOpt => passphraseOption = newOpt"
                        @passphrase-changed="newPass => passphrase = newPass"
                    />
                </v-window-item>

                <v-window-item :value="CreateAccessStep.EnterNewPassphrase">
                    <enter-passphrase-step
                        :ref="stepInfos[CreateAccessStep.EnterNewPassphrase].ref"
                        :passphrase-type="CreateAccessStep.EnterNewPassphrase"
                        @passphrase-changed="newPass => passphrase = newPass"
                    >
                        This passphrase will be used to encrypt all the files you upload using this access grant.
                        You will need it to access these files in the future.
                    </enter-passphrase-step>
                </v-window-item>

                <v-window-item :value="CreateAccessStep.PassphraseGenerated">
                    <passphrase-generated-step
                        :ref="stepInfos[CreateAccessStep.PassphraseGenerated].ref"
                        :name="name"
                        @passphrase-changed="newPass => passphrase = newPass"
                    >
                        This passphrase will be used to encrypt all the files you upload using this access grant.
                        You will need it to access these files in the future.
                    </passphrase-generated-step>
                </v-window-item>

                <v-window-item :value="CreateAccessStep.ConfirmDetails">
                    <confirm-details-step
                        :ref="stepInfos[CreateAccessStep.ConfirmDetails].ref"
                        :name="name"
                        :types="accessTypes"
                        :permissions="permissions"
                        :buckets="buckets"
                        :end-date="(endDate as AccessGrantEndDate)"
                    />
                </v-window-item>

                <v-window-item :value="CreateAccessStep.AccessCreated">
                    <access-created-step
                        :ref="stepInfos[CreateAccessStep.AccessCreated].ref"
                        :name="name"
                        :access-grant="accessGrant"
                    />
                </v-window-item>

                <v-window-item :value="CreateAccessStep.CLIAccessCreated">
                    <c-l-i-access-created-step
                        :ref="stepInfos[CreateAccessStep.CLIAccessCreated].ref"
                        :name="name"
                        :api-key="cliAccess"
                    />
                </v-window-item>

                <v-window-item :value="CreateAccessStep.CredentialsCreated">
                    <s3-credentials-created-step
                        :ref="stepInfos[CreateAccessStep.CredentialsCreated].ref"
                        :name="name"
                        :access-key="edgeCredentials.accessKeyId"
                        :secret-key="edgeCredentials.secretKey"
                        :endpoint="edgeCredentials.endpoint"
                    />
                </v-window-item>
            </v-window>

            <v-divider />

            <v-card-actions class="pa-7">
                <v-row>
                    <v-col>
                        <v-btn
                            v-bind="stepInfos[step].prev.value ? undefined : {
                                'href': stepInfos[step].docsLink || 'https://docs.storj.io/dcs/access',
                                'target': '_blank',
                                'rel': 'noopener noreferrer',
                            }"
                            variant="outlined"
                            color="default"
                            block
                            :prepend-icon="stepInfos[step].prev.value ? mdiChevronLeft : mdiBookOpenOutline"
                            :disabled="isCreating || isFetching"
                            @click="prevStep"
                        >
                            {{ stepInfos[step].prev.value ? 'Back' : 'Learn More' }}
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary"
                            variant="flat"
                            block
                            :append-icon="stepInfos[step].next.value ? mdiChevronRight : undefined"
                            :loading="isCreating"
                            :disabled="isFetching"
                            @click="nextStep"
                        >
                            {{ stepInfos[step].nextText.value }}
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { Component, Ref, computed, ref, watch, WatchStopHandle } from 'vue';
import {
    VCol,
    VRow,
    VBtn,
    VDialog,
    VCard,
    VCardItem,
    VCardTitle,
    VDivider,
    VWindow,
    VWindowItem,
    VCardActions,
    VProgressLinear,
} from 'vuetify/components';
import { mdiBookOpenOutline, mdiChevronLeft, mdiChevronRight } from '@mdi/js';

import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useNotify } from '@/utils/hooks';
import { AccessType, PassphraseOption, Permission, CreateAccessStep, STEP_ICON_AND_TITLE } from '@/types/createAccessGrant';
import { AccessGrantEndDate, ACCESS_TYPE_LINKS } from '@poc/types/createAccessGrant';
import { LocalData } from '@/utils/localData';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { DialogStepComponent } from '@poc/types/common';

import CreateNewAccessStep from '@poc/components/dialogs/createAccessSteps/CreateNewAccessStep.vue';
import ChoosePermissionsStep from '@poc/components/dialogs/createAccessSteps/ChoosePermissionsStep.vue';
import AccessEncryptionStep from '@poc/components/dialogs/createAccessSteps/AccessEncryptionStep.vue';
import EncryptionInfoStep from '@poc/components/dialogs/createAccessSteps/EncryptionInfoStep.vue';
import EnterPassphraseStep from '@poc/components/dialogs/commonPassphraseSteps/EnterPassphraseStep.vue';
import PassphraseGeneratedStep from '@poc/components/dialogs/commonPassphraseSteps/PassphraseGeneratedStep.vue';
import ConfirmDetailsStep from '@poc/components/dialogs/createAccessSteps/ConfirmDetailsStep.vue';
import AccessCreatedStep from '@poc/components/dialogs/createAccessSteps/AccessCreatedStep.vue';
import CLIAccessCreatedStep from '@poc/components/dialogs/createAccessSteps/CLIAccessCreatedStep.vue';
import S3CredentialsCreatedStep from '@poc/components/dialogs/createAccessSteps/S3CredentialsCreatedStep.vue';

type CreateAccessLocation = CreateAccessStep | null | (() => (CreateAccessStep | null));

class StepInfo {
    public ref: Ref<DialogStepComponent | null> = ref<DialogStepComponent | null>(null);
    public prev: Ref<CreateAccessStep | null>;
    public next: Ref<CreateAccessStep | null>;
    public nextText: Ref<string>;

    constructor(
        prev: CreateAccessLocation = null,
        next: CreateAccessLocation = null,
        public docsLink: string | null = null,
        nextText: string | (() => string) = 'Next',
        public beforeNext?: () => Promise<boolean>,
    ) {
        this.prev = (typeof prev === 'function') ? computed<CreateAccessStep | null>(prev) : ref<CreateAccessStep | null>(prev);
        this.next = (typeof next === 'function') ? computed<CreateAccessStep | null>(next) : ref<CreateAccessStep | null>(next);
        this.nextText = (typeof nextText === 'function') ? computed<string>(nextText) : ref<string>(nextText);
    }
}

const resets: (() => void)[] = [];

function resettableRef<T>(value: T): Ref<T> {
    const thisRef = ref<T>(value) as Ref<T>;
    resets.push(() => thisRef.value = value);
    return thisRef;
}

const props = defineProps<{
    modelValue: boolean,
}>();

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => {
        if (isCreating.value) return;
        emit('update:modelValue', value);
    },
});

const emit = defineEmits<{
    'update:modelValue': [value: boolean];
}>();

const bucketsStore = useBucketsStore();
const projectsStore = useProjectsStore();
const agStore = useAccessGrantsStore();
const configStore = useConfigStore();
const notify = useNotify();
const analyticsStore = useAnalyticsStore();

const innerContent = ref<Component | null>(null);
const step = resettableRef<CreateAccessStep>(CreateAccessStep.CreateNewAccess);
const isFetching = ref<boolean>(true);

// Create New Access
const name = resettableRef<string>('');
const accessTypes = resettableRef<AccessType[]>([]);

// Permissions
const permissions = resettableRef<Permission[]>([]);
const buckets = resettableRef<string[]>([]);
const endDate = resettableRef<AccessGrantEndDate | null>(null);

// Select Passphrase Type
const passphraseOption = resettableRef<PassphraseOption | null>(null);

// Enter / Generate Passphrase
const passphrase = resettableRef<string>('');

// Confirm Details
const isCreating = ref<boolean>(false);

// Access Created
const accessGrant = resettableRef<string>('');

// S3 Credentials Created
const edgeCredentials = resettableRef<EdgeCredentials>(new EdgeCredentials());

// CLI Access Created
const cliAccess = resettableRef<string>('');

const worker = ref<Worker | null>(null);

const stepInfos: Record<CreateAccessStep, StepInfo> = {
    [CreateAccessStep.CreateNewAccess]: new StepInfo(
        null,
        () => (accessTypes.value.includes(AccessType.S3) && !LocalData.getServerSideEncryptionModalHidden())
            ? CreateAccessStep.EncryptionInfo
            : CreateAccessStep.ChoosePermission,
    ),
    [CreateAccessStep.EncryptionInfo]: new StepInfo(
        CreateAccessStep.CreateNewAccess,
        CreateAccessStep.ChoosePermission,
    ),
    [CreateAccessStep.ChoosePermission]: new StepInfo(
        () => (accessTypes.value.includes(AccessType.S3) && !LocalData.getServerSideEncryptionModalHidden())
            ? CreateAccessStep.EncryptionInfo
            : CreateAccessStep.CreateNewAccess,
        () => accessTypes.value.includes(AccessType.APIKey) ? CreateAccessStep.ConfirmDetails : CreateAccessStep.AccessEncryption,
    ),
    [CreateAccessStep.AccessEncryption]: new StepInfo(
        CreateAccessStep.ChoosePermission,
        () => {
            switch (passphraseOption.value) {
            case PassphraseOption.EnterNewPassphrase: return CreateAccessStep.EnterNewPassphrase;
            case PassphraseOption.GenerateNewPassphrase: return CreateAccessStep.PassphraseGenerated;
            default: return CreateAccessStep.ConfirmDetails;
            }
        },
    ),

    [CreateAccessStep.EnterMyPassphrase]: new StepInfo(), // unused

    [CreateAccessStep.EnterNewPassphrase]: new StepInfo(
        CreateAccessStep.AccessEncryption,
        CreateAccessStep.ConfirmDetails,
    ),
    [CreateAccessStep.PassphraseGenerated]: new StepInfo(
        CreateAccessStep.AccessEncryption,
        CreateAccessStep.ConfirmDetails,
    ),

    [CreateAccessStep.ConfirmDetails]: new StepInfo(
        () => {
            switch (passphraseOption.value) {
            case PassphraseOption.EnterNewPassphrase: return CreateAccessStep.EnterNewPassphrase;
            case PassphraseOption.GenerateNewPassphrase: return CreateAccessStep.PassphraseGenerated;
            default: return CreateAccessStep.AccessEncryption;
            }
        },
        () => {
            if (accessTypes.value.includes(AccessType.AccessGrant)) return CreateAccessStep.AccessCreated;
            if (accessTypes.value.includes(AccessType.S3)) return CreateAccessStep.CredentialsCreated;
            return CreateAccessStep.CLIAccessCreated;
        },
        null,
        'Create Access',
        async () => {
            isCreating.value = true;

            try {
                await createCLIAccess();
                if (accessTypes.value.includes(AccessType.AccessGrant) || accessTypes.value.includes(AccessType.S3)) {
                    await createAccessGrant();
                }
                if (accessTypes.value.includes(AccessType.S3)) {
                    await createEdgeCredentials();
                }
            } catch (error) {
                notify.error(`Error creating access grant. ${error.message}`, AnalyticsErrorEventSource.CREATE_AG_MODAL);
                isCreating.value = false;
                return false;
            }

            // This is an action to handle case if user sets project level passphrase.
            if (
                passphraseOption.value === PassphraseOption.SetMyProjectPassphrase &&
                !accessTypes.value.includes(AccessType.APIKey)
            ) {
                bucketsStore.setEdgeCredentials(new EdgeCredentials());
                bucketsStore.setPassphrase(passphrase.value);
                bucketsStore.setPromptForPassphrase(false);
            }

            isCreating.value = false;

            return true;
        },
    ),

    [CreateAccessStep.AccessCreated]: new StepInfo(
        null,
        () => (accessTypes.value.includes(AccessType.S3)) ? CreateAccessStep.CredentialsCreated : null,
        ACCESS_TYPE_LINKS[AccessType.AccessGrant],
        () => (accessTypes.value.includes(AccessType.S3)) ? 'Next' : 'Finish',
    ),
    [CreateAccessStep.CredentialsCreated]: new StepInfo(null, null, ACCESS_TYPE_LINKS[AccessType.S3], 'Finish'),
    [CreateAccessStep.CLIAccessCreated]: new StepInfo(null, null, ACCESS_TYPE_LINKS[AccessType.APIKey], 'Finish'),
};

/**
 * Navigates to the next step.
 */
async function nextStep(): Promise<void> {
    const info = stepInfos[step.value];

    if (isCreating.value || isFetching.value || info.ref.value?.validate?.() === false) return;

    if (info.beforeNext) try {
        if (!await info.beforeNext()) return;
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.CREATE_AG_MODAL);
        return;
    }

    info.ref.value?.onExit?.('next');

    const next = info.next.value;
    if (!next) {
        model.value = false;
        return;
    }
    step.value = next;
}

/**
 * Navigates to the previous step.
 */
function prevStep(): void {
    const info = stepInfos[step.value];

    info.ref.value?.onExit?.('prev');

    const prev = info.prev.value;
    if (!prev) return;
    step.value = prev;
}

/**
 * Initializes the current step when it has changed.
 */
watch(step, newStep => {
    if (!innerContent.value) return;

    // Window items are lazy loaded, so the component may not exist yet
    let unwatch: WatchStopHandle | null = null;
    let unwatchImmediately = false;
    unwatch = watch(
        () => stepInfos[newStep].ref.value,
        stepComp => {
            if (!stepComp) return;
            stepComp.onEnter?.();
            if (unwatch) {
                unwatch();
                return;
            }
            unwatchImmediately = true;
        },
        { immediate: true },
    );
    if (unwatchImmediately) unwatch();
});

/**
 * Generates CLI access.
 */
async function createCLIAccess(): Promise<void> {
    if (!worker.value) {
        throw new Error('Web worker is not initialized.');
    }

    const projectID = projectsStore.state.selectedProject.id;

    // creates fresh new API key.
    const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(name.value, projectID);

    await agStore.getAccessGrants(1, projectID).catch(err => {
        notify.error(`Unable to fetch access grants. ${err.message}`, AnalyticsErrorEventSource.CREATE_AG_MODAL);
    });

    let permissionsMsg = {
        'type': 'SetPermission',
        'buckets': JSON.stringify(buckets.value),
        'apiKey': cleanAPIKey.secret,
        'isDownload': permissions.value.includes(Permission.Read),
        'isUpload': permissions.value.includes(Permission.Write),
        'isList': permissions.value.includes(Permission.List),
        'isDelete': permissions.value.includes(Permission.Delete),
        'notBefore': new Date().toISOString(),
    };

    const notAfter = endDate.value?.date;
    if (notAfter) permissionsMsg = Object.assign(permissionsMsg, { 'notAfter': notAfter.toISOString() });

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

    if (accessTypes.value.includes(AccessType.APIKey)) {
        analyticsStore.eventTriggered(AnalyticsEvent.API_ACCESS_CREATED);
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
    const satelliteNodeURL = configStore.state.config.satelliteNodeURL;

    const salt = await projectsStore.getProjectSalt(projectsStore.state.selectedProject.id);

    if (!passphrase.value) {
        throw new Error('Passphrase can\'t be empty');
    }

    worker.value.postMessage({
        'type': 'GenerateAccess',
        'apiKey': cliAccess.value,
        'passphrase': passphrase.value,
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

    if (accessTypes.value.includes(AccessType.AccessGrant)) {
        analyticsStore.eventTriggered(AnalyticsEvent.ACCESS_GRANT_CREATED);
    }
}

/**
 * Generates edge credentials.
 */
async function createEdgeCredentials(): Promise<void> {
    edgeCredentials.value = await agStore.getEdgeCredentials(accessGrant.value);
    analyticsStore.eventTriggered(AnalyticsEvent.GATEWAY_CREDENTIALS_CREATED);
}

/**
 * Executes when the dialog's inner content has been added or removed.
 * If removed, refs are reset back to their initial values.
 * Otherwise, data is fetched and the current step is initialized.
 *
 * This is used instead of onMounted because the dialog remains mounted
 * even when hidden.
 */
watch(innerContent, async (comp: Component): Promise<void> => {
    if (!comp) {
        resets.forEach(reset => reset());
        return;
    }

    worker.value = agStore.state.accessGrantsWebWorker;
    if (worker.value) {
        worker.value.onerror = (error: ErrorEvent) => {
            notify.error(error.message, AnalyticsErrorEventSource.CREATE_AG_MODAL);
        };
    }

    isFetching.value = true;

    const projectID = projectsStore.state.selectedProject.id;
    await agStore.getAllAGNames(projectID).catch(err => {
        notify.error(`Error fetching access grant names. ${err.message}`, AnalyticsErrorEventSource.CREATE_AG_MODAL);
    });
    await bucketsStore.getAllBucketsNames(projectID).catch(err => {
        notify.error(`Error fetching bucket grant names. ${err.message}`, AnalyticsErrorEventSource.CREATE_AG_MODAL);
    });

    isFetching.value = false;

    stepInfos[step.value].ref.value?.onEnter?.();
});
</script>

<style scoped lang="scss">
.create-access-dialog__window {
    transition: opacity 250ms cubic-bezier(0.4, 0, 0.2, 1);

    &--loading {
        opacity: 0.3;
        transition: opacity 0s;
        pointer-events: none;
    }
}
</style>

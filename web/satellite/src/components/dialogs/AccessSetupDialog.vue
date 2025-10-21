// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        max-width="450px"
        min-width="320px"
        transition="fade-transition"
        persistent
        scrollable
    >
        <v-card ref="innerContent">
            <v-sheet>
                <v-card-item class="pa-6">
                    <v-card-title class="font-weight-bold mt-n1">
                        New {{ selectedApp ? selectedApp.name : '' }} Access
                    </v-card-title>
                    <v-card-subtitle class="text-caption pb-0">
                        {{ stepName }}
                    </v-card-subtitle>
                    <template #append>
                        <v-btn
                            :icon="X"
                            variant="text"
                            size="small"
                            color="default"
                            :disabled="isCreating"
                            @click="model = false"
                        />
                    </template>
                    <template v-if="selectedApp" #prepend>
                        <img :src="selectedApp.src" :alt="selectedApp.name" width="40" height="40" class="rounded-md border pa-2">
                    </template>
                    <template v-else #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <component :is="KeyRound" :size="18" />
                        </v-sheet>
                    </template>
                    <v-progress-linear height="2px" indeterminate absolute location="bottom" :active="isFetching || isCreating" />
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-card-text class="pa-0">
                <v-window
                    v-model="step"
                    :touch="false"
                    class="setup-app__window"
                    :class="{ 'setup-app__window--loading': isFetching }"
                >
                    <v-window-item :value="SetupStep.ChooseAccessStep">
                        <choose-access-step
                            :ref="stepInfos[SetupStep.ChooseAccessStep].ref"
                            @name-changed="newName => name = newName"
                            @type-changed="newType => accessType = newType"
                            @submit="nextStep"
                            @app-changed="application => selectedApp = application"
                        />
                    </v-window-item>

                    <v-window-item :value="SetupStep.EncryptionInfo">
                        <encryption-info-step :ref="stepInfos[SetupStep.EncryptionInfo].ref" />
                    </v-window-item>

                    <v-window-item :value="SetupStep.ChooseFlowStep">
                        <choose-flow-step
                            :ref="stepInfos[SetupStep.ChooseFlowStep].ref"
                            :app="selectedApp"
                            @set-flow-type="val => flowType = val"
                        />
                    </v-window-item>

                    <v-window-item :value="SetupStep.AccessEncryption">
                        <access-encryption-step
                            :ref="stepInfos[SetupStep.AccessEncryption].ref"
                            @select-option="val => passphraseOption = val"
                            @passphrase-changed="val => passphrase = val"
                            @submit="nextStep"
                        />
                    </v-window-item>

                    <v-window-item :value="SetupStep.EnterNewPassphrase">
                        <enter-passphrase-step
                            :ref="stepInfos[SetupStep.EnterNewPassphrase].ref"
                            @passphrase-changed="val => passphrase = val"
                        />
                    </v-window-item>

                    <v-window-item :value="SetupStep.PassphraseGenerated">
                        <passphrase-generated-step
                            :ref="stepInfos[SetupStep.PassphraseGenerated].ref"
                            :name="name"
                            @passphrase-changed="val => passphrase = val"
                        />
                    </v-window-item>

                    <v-window-item :value="SetupStep.ChoosePermissionsStep">
                        <choose-permissions-step
                            :ref="stepInfos[SetupStep.ChoosePermissionsStep].ref"
                            @permissions-changed="val => permissions = val"
                        />
                    </v-window-item>

                    <v-window-item :value="SetupStep.ObjectLockPermissionsStep">
                        <object-lock-permissions-step
                            :ref="stepInfos[SetupStep.ObjectLockPermissionsStep].ref"
                            @permissions-changed="val => objectLockPermissions = val"
                        />
                    </v-window-item>

                    <v-window-item :value="SetupStep.SelectBucketsStep">
                        <select-buckets-step
                            :ref="stepInfos[SetupStep.SelectBucketsStep].ref"
                            @buckets-changed="val => buckets = val"
                        />
                    </v-window-item>

                    <v-window-item :value="SetupStep.OptionalExpirationStep">
                        <optional-expiration-step
                            :ref="stepInfos[SetupStep.OptionalExpirationStep].ref"
                            :end-date="endDate"
                            @end-date-changed="val => endDate = val"
                        />
                    </v-window-item>

                    <v-window-item :value="SetupStep.ConfirmDetailsStep">
                        <confirm-details-step
                            :ref="stepInfos[SetupStep.ConfirmDetailsStep].ref"
                            :name="name"
                            :type="accessType"
                            :permissions="permissions"
                            :object-lock-permissions="objectLockPermissions"
                            :buckets="buckets"
                            :end-date="endDate"
                        />
                    </v-window-item>

                    <v-window-item :value="SetupStep.AccessCreatedStep">
                        <access-created-step
                            :ref="stepInfos[SetupStep.AccessCreatedStep].ref"
                            :name="name"
                            :app="selectedApp"
                            :cli-access="cliAccess"
                            :access-grant="accessGrant"
                            :credentials="credentials"
                            :access-type="accessType"
                        />
                    </v-window-item>
                </v-window>
            </v-card-text>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            :disabled="isCreating || isFetching"
                            @click="prevStep"
                        >
                            {{ stepInfos[step].prevText.value }}
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            v-if="step !== SetupStep.AccessCreatedStep"
                            color="primary"
                            variant="flat"
                            block
                            :loading="isCreating"
                            :disabled="isFetching"
                            @click="nextStep"
                        >
                            {{ stepInfos[step].nextText.value }}
                        </v-btn>
                        <v-btn
                            v-else
                            color="primary"
                            variant="flat"
                            block
                            :href="selectedApp ? selectedApp.docs : docsLink"
                            target="_blank"
                            rel="noopener noreferrer"
                            @click="() => sendApplicationsAnalytics(AnalyticsEvent.APPLICATIONS_DOCS_CLICKED)"
                        >
                            <template #prepend>
                                <component :is="BookOpenText" :size="18" />
                            </template>
                            Read Docs
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, Ref, ref, watch, WatchStopHandle } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardSubtitle,
    VCardText,
    VCardTitle,
    VCol,
    VDialog,
    VDivider,
    VProgressLinear,
    VRow,
    VSheet,
    VWindow,
    VWindowItem,
} from 'vuetify/components';
import { useRoute } from 'vue-router';
import { BookOpenText, KeyRound, X } from 'lucide-vue-next';

import {
    AccessType,
    FlowType,
    ObjectLockPermission,
    PassphraseOption,
    Permission,
    SetupStep,
} from '@/types/setupAccess';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { getUniqueName, IDialogFlowStep } from '@/types/common';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { ROUTES } from '@/router';
import { useUsersStore } from '@/store/modules/usersStore';
import { Application } from '@/types/applications';
import { SetPermissionsMessage, useAccessGrantWorker } from '@/composables/useAccessGrantWorker';

import ChooseFlowStep from '@/components/dialogs/accessSetupSteps/ChooseFlowStep.vue';
import ChooseAccessStep from '@/components/dialogs/accessSetupSteps/ChooseAccessStep.vue';
import ChoosePermissionsStep from '@/components/dialogs/accessSetupSteps/ChoosePermissionsStep.vue';
import SelectBucketsStep from '@/components/dialogs/accessSetupSteps/SelectBucketsStep.vue';
import AccessCreatedStep from '@/components/dialogs/accessSetupSteps/AccessCreatedStep.vue';
import AccessEncryptionStep from '@/components/dialogs/accessSetupSteps/AccessEncryptionStep.vue';
import EnterPassphraseStep from '@/components/dialogs/commonPassphraseSteps/EnterPassphraseStep.vue';
import PassphraseGeneratedStep from '@/components/dialogs/commonPassphraseSteps/PassphraseGeneratedStep.vue';
import OptionalExpirationStep from '@/components/dialogs/accessSetupSteps/OptionalExpirationStep.vue';
import EncryptionInfoStep from '@/components/dialogs/accessSetupSteps/EncryptionInfoStep.vue';
import ConfirmDetailsStep from '@/components/dialogs/accessSetupSteps/ConfirmDetailsStep.vue';
import ObjectLockPermissionsStep from '@/components/dialogs/accessSetupSteps/ObjectLockPermissionsStep.vue';

type SetupLocation = SetupStep | undefined | (() => (SetupStep | undefined));

class StepInfo {
    public ref = ref<IDialogFlowStep>();
    public prev: Ref<SetupStep | undefined>;
    public next: Ref<SetupStep | undefined>;
    public nextText: Ref<string>;
    public prevText: Ref<string>;

    constructor(
        nextText: string | (() => string),
        prevText: string | (() => string),
        prev: SetupLocation = undefined,
        next: SetupLocation = undefined,
        public beforeNext?: () => void | Promise<void>,
    ) {
        this.prev = (typeof prev === 'function') ? computed<SetupStep | undefined>(prev) : ref<SetupStep | undefined>(prev);
        this.next = (typeof next === 'function') ? computed<SetupStep | undefined>(next) : ref<SetupStep | undefined>(next);
        this.nextText = (typeof nextText === 'function') ? computed<string>(nextText) : ref<string>(nextText);
        this.prevText = (typeof prevText === 'function') ? computed<string>(prevText) : ref<string>(prevText);
    }
}

const emit = defineEmits<{
    'accessCreated': [];
}>();

const props = withDefaults(defineProps<{
    docsLink: string
    defaultStep: SetupStep
    app?: Application
    accessName?: string
    defaultAccessType?: AccessType
}>(), {
    app: undefined,
    accessName: undefined,
    defaultAccessType: undefined,
});

const agStore = useAccessGrantsStore();
const bucketsStore = useBucketsStore();
const projectsStore = useProjectsStore();
const configStore = useConfigStore();
const analyticsStore = useAnalyticsStore();
const userStore = useUsersStore();

const notify = useNotify();
const route = useRoute();
const { setPermissions, generateAccess } = useAccessGrantWorker();

const model = defineModel<boolean>({ required: true });

const innerContent = ref<VCard>();
const isCreating = ref<boolean>(false);
const isFetching = ref<boolean>(true);

const resets: (() => void)[] = [];
function resettableRef<T>(value: T): Ref<T> {
    const thisRef = ref<T>(value) as Ref<T>;
    resets.push(() => thisRef.value = value);
    return thisRef;
}

const selectedApp = resettableRef<Application | undefined>(props.app);
const step = resettableRef<SetupStep>(props.defaultStep);
const accessType = resettableRef<AccessType>(props.defaultAccessType ?? AccessType.S3);
const flowType = resettableRef<FlowType>(FlowType.FullAccess);
const name = resettableRef<string>('');
const permissions = resettableRef<Permission[]>([]);
const objectLockPermissions = resettableRef<ObjectLockPermission[]>([]);
const buckets = resettableRef<string[]>([]);
const passphrase = resettableRef<string>(bucketsStore.state.passphrase);
const endDate = resettableRef<Date | null>(null);
const passphraseOption = resettableRef<PassphraseOption>(PassphraseOption.EnterNewPassphrase);
const cliAccess = resettableRef<string>('');
const accessGrant = resettableRef<string>('');
const credentials = resettableRef<EdgeCredentials>(new EdgeCredentials());

const promptForPassphrase = computed<boolean>(() => bucketsStore.state.promptForPassphrase);

const hasManagedPassphrase = computed<boolean>(() => projectsStore.state.selectedProjectConfig.hasManagedPassphrase);

const stepName = computed<string>(() => {
    switch (step.value) {
    case SetupStep.ChooseAccessStep:
        return 'Access Name and Type';
    case SetupStep.EncryptionInfo:
        return 'Encryption Information';
    case SetupStep.ChooseFlowStep:
        return 'Configure Access';
    case SetupStep.AccessEncryption:
        return 'Access Encryption';
    case SetupStep.EnterNewPassphrase:
        return 'New Passphrase';
    case SetupStep.PassphraseGenerated:
        return 'Passphrase Generated';
    case SetupStep.ChoosePermissionsStep:
        return 'Access Permissions';
    case SetupStep.ObjectLockPermissionsStep:
        return 'Object Lock Permissions';
    case SetupStep.SelectBucketsStep:
        return 'Bucket Restrictions';
    case SetupStep.OptionalExpirationStep:
        return 'Access Expiration';
    case SetupStep.ConfirmDetailsStep:
        return 'Confirm Access Details';
    case SetupStep.AccessCreatedStep:
        return 'Access Created Successfully';
    default:
        return '';
    }
});

/**
 * Whether object lock UI is enabled.
 */
const objectLockUIEnabled = computed<boolean>(() => configStore.state.config.objectLockUIEnabled);

const stepInfos: Record<SetupStep, StepInfo> = {
    [SetupStep.ChooseAccessStep]: new StepInfo(
        'Next ->',
        'Cancel',
        undefined,
        () => (accessType.value === AccessType.S3 && !userStore.noticeDismissal.serverSideEncryption && !hasManagedPassphrase.value)
            ? SetupStep.EncryptionInfo
            : SetupStep.ChooseFlowStep,
    ),
    [SetupStep.EncryptionInfo]: new StepInfo(
        'Next ->',
        'Back',
        SetupStep.ChooseAccessStep,
        SetupStep.ChooseFlowStep,
    ),
    [SetupStep.ChooseFlowStep]: new StepInfo(
        'Next ->',
        () => props.defaultAccessType ? 'Cancel' : 'Back',
        () => {
            if (props.defaultAccessType) return undefined;

            return accessType.value === AccessType.S3 && !userStore.noticeDismissal.serverSideEncryption  && !hasManagedPassphrase.value
                ? SetupStep.EncryptionInfo
                : SetupStep.ChooseAccessStep;
        },
        () => {
            if (accessType.value === AccessType.APIKey) {
                return flowType.value === FlowType.FullAccess ? SetupStep.ConfirmDetailsStep : SetupStep.ChoosePermissionsStep;
            }

            if (promptForPassphrase.value) return SetupStep.AccessEncryption;

            return flowType.value === FlowType.FullAccess ? SetupStep.ConfirmDetailsStep : SetupStep.ChoosePermissionsStep;
        },
        () => {
            if (flowType.value === FlowType.FullAccess) setFullAccess();
        },
    ),
    [SetupStep.AccessEncryption]: new StepInfo(
        'Next ->',
        'Back',
        SetupStep.ChooseFlowStep,
        () => {
            if (passphraseOption.value === PassphraseOption.EnterNewPassphrase) return SetupStep.EnterNewPassphrase;
            if (passphraseOption.value === PassphraseOption.GenerateNewPassphrase) return SetupStep.PassphraseGenerated;

            return flowType.value === FlowType.FullAccess ? SetupStep.ConfirmDetailsStep : SetupStep.ChoosePermissionsStep;
        },
        () => {
            if (flowType.value === FlowType.FullAccess) setFullAccess();
        },
    ),
    [SetupStep.PassphraseGenerated]: new StepInfo(
        'Next ->',
        'Back',
        SetupStep.AccessEncryption,
        () => flowType.value === FlowType.FullAccess ? SetupStep.ConfirmDetailsStep : SetupStep.ChoosePermissionsStep,
        () => {
            if (flowType.value === FlowType.FullAccess) setFullAccess();
        },
    ),
    [SetupStep.EnterNewPassphrase]: new StepInfo(
        'Next ->',
        'Back',
        SetupStep.AccessEncryption,
        () => flowType.value === FlowType.FullAccess ? SetupStep.ConfirmDetailsStep : SetupStep.ChoosePermissionsStep,
        () => {
            if (flowType.value === FlowType.FullAccess) setFullAccess();
        },
    ),
    [SetupStep.ChoosePermissionsStep]: new StepInfo(
        'Next ->',
        'Back',
        () => {
            if (bucketsStore.state.promptForPassphrase && accessType.value !== AccessType.APIKey) {
                if (passphraseOption.value === PassphraseOption.EnterNewPassphrase) return SetupStep.EnterNewPassphrase;
                if (passphraseOption.value === PassphraseOption.GenerateNewPassphrase) return SetupStep.PassphraseGenerated;

                return SetupStep.AccessEncryption;
            }

            return SetupStep.ChooseFlowStep;
        },
        () => objectLockUIEnabled.value ? SetupStep.ObjectLockPermissionsStep : SetupStep.SelectBucketsStep,
    ),
    [SetupStep.ObjectLockPermissionsStep]: new StepInfo(
        'Next ->',
        'Back',
        SetupStep.ChoosePermissionsStep,
        SetupStep.SelectBucketsStep,
    ),
    [SetupStep.SelectBucketsStep]: new StepInfo(
        'Next ->',
        'Back',
        () => objectLockUIEnabled.value ? SetupStep.ObjectLockPermissionsStep : SetupStep.ChoosePermissionsStep,
        SetupStep.OptionalExpirationStep,
    ),
    [SetupStep.OptionalExpirationStep]: new StepInfo(
        'Next ->',
        'Back',
        SetupStep.SelectBucketsStep,
        SetupStep.ConfirmDetailsStep,
    ),
    [SetupStep.ConfirmDetailsStep]: new StepInfo(
        'Create Access',
        'Back',
        () => {
            if (flowType.value === FlowType.FullAccess) {
                if (bucketsStore.state.promptForPassphrase && accessType.value !== AccessType.APIKey) {
                    if (passphraseOption.value === PassphraseOption.EnterNewPassphrase) return SetupStep.EnterNewPassphrase;
                    if (passphraseOption.value === PassphraseOption.GenerateNewPassphrase) return SetupStep.PassphraseGenerated;

                    return SetupStep.AccessEncryption;
                }

                return SetupStep.ChooseFlowStep;
            }

            return SetupStep.OptionalExpirationStep;
        },
        SetupStep.AccessCreatedStep,
        generate,
    ),
    [SetupStep.AccessCreatedStep]: new StepInfo('', 'Close'),
};

/**
 * Set unique name of access to be created.
 */
function setDefaultName(): void {
    if (!props.accessName) return;

    name.value = getUniqueName(props.accessName, agStore.state.allAGNames);
}

/**
 * Generates access based on selected properties.
 */
async function generate(): Promise<void> {
    if (isCreating.value) return;

    isCreating.value = true;

    await createAPIKey();
    if (accessType.value === AccessType.AccessGrant || accessType.value === AccessType.S3) {
        await createAccessGrant();
        if (accessType.value === AccessType.S3) await createEdgeCredentials();
        if (passphraseOption.value === PassphraseOption.SetMyProjectPassphrase) {
            bucketsStore.setEdgeCredentials(new EdgeCredentials());
            bucketsStore.setPassphrase(passphrase.value);
            bucketsStore.setPromptForPassphrase(false);
        }
    }

    if (selectedApp.value) sendApplicationsAnalytics(AnalyticsEvent.APPLICATIONS_SETUP_COMPLETED);

    isCreating.value = false;
}

/**
 * Generates API Key.
 */
async function createAPIKey(): Promise<void> {
    const projectID = projectsStore.state.selectedProject.id;
    const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(name.value, projectID);

    if (route.name === ROUTES.Access.name) {
        agStore.getAccessGrants(1, projectID).catch(error => {
            notify.notifyError(error, AnalyticsErrorEventSource.SETUP_ACCESS_MODAL);
        });
    }

    const noCaveats = flowType.value === FlowType.FullAccess;

    let permissionsMsg: SetPermissionsMessage = {
        buckets: JSON.stringify(noCaveats ? [] : buckets.value),
        apiKey: cleanAPIKey.secret,
        isDownload: noCaveats || permissions.value.includes(Permission.Read),
        isUpload: noCaveats || permissions.value.includes(Permission.Write),
        isList: noCaveats || permissions.value.includes(Permission.List),
        isDelete: noCaveats || permissions.value.includes(Permission.Delete),
        notBefore: new Date().toISOString(),
    };

    if (objectLockUIEnabled.value) {
        permissionsMsg = {
            ...permissionsMsg,
            isPutObjectRetention: noCaveats || objectLockPermissions.value.includes(ObjectLockPermission.PutObjectRetention),
            isGetObjectRetention: noCaveats || objectLockPermissions.value.includes(ObjectLockPermission.GetObjectRetention),
            isBypassGovernanceRetention: objectLockPermissions.value.includes(ObjectLockPermission.BypassGovernanceRetention),
            isPutObjectLegalHold: noCaveats || objectLockPermissions.value.includes(ObjectLockPermission.PutObjectLegalHold),
            isGetObjectLegalHold: noCaveats || objectLockPermissions.value.includes(ObjectLockPermission.GetObjectLegalHold),
            isPutObjectLockConfiguration: noCaveats || objectLockPermissions.value.includes(ObjectLockPermission.PutObjectLockConfiguration),
            isGetObjectLockConfiguration: noCaveats || objectLockPermissions.value.includes(ObjectLockPermission.GetObjectLockConfiguration),
        };
    }

    if (endDate.value && !noCaveats) permissionsMsg = Object.assign(permissionsMsg, { notAfter: endDate.value.toISOString() });

    cliAccess.value = await setPermissions(permissionsMsg);

    if (accessType.value === AccessType.APIKey)
        analyticsStore.eventTriggered(AnalyticsEvent.API_ACCESS_CREATED, { project_id: projectID });
}

/**
 * Generates access grant.
 */
async function createAccessGrant(): Promise<void> {
    if (!passphrase.value) throw new Error('Passphrase can\'t be empty');

    accessGrant.value = await generateAccess({
        apiKey: cliAccess.value,
        passphrase: passphrase.value,
    }, projectsStore.state.selectedProject.id);

    if (accessType.value === AccessType.AccessGrant)
        analyticsStore.eventTriggered(AnalyticsEvent.ACCESS_GRANT_CREATED, { project_id: projectsStore.state.selectedProject.id });
}

/**
 * Generates edge credentials.
 */
async function createEdgeCredentials(): Promise<void> {
    credentials.value = await agStore.getEdgeCredentials(accessGrant.value);
    analyticsStore.eventTriggered(AnalyticsEvent.GATEWAY_CREDENTIALS_CREATED, { project_id: projectsStore.state.selectedProject.id });
}

function setFullAccess(): void {
    permissions.value = [Permission.Read, Permission.Write, Permission.List, Permission.Delete];
    objectLockPermissions.value = [
        ObjectLockPermission.PutObjectRetention,
        ObjectLockPermission.GetObjectRetention,
        ObjectLockPermission.PutObjectLegalHold,
        ObjectLockPermission.GetObjectLegalHold,
        ObjectLockPermission.PutObjectLockConfiguration,
        ObjectLockPermission.GetObjectLockConfiguration,
    ];
}

/**
 * Navigates to the next step.
 */
async function nextStep(): Promise<void> {
    const info = stepInfos[step.value];

    if (isCreating.value || isFetching.value || info.ref.value?.validate?.() === false) return;

    info.ref.value?.onExit?.('next');

    if (info.beforeNext) {
        try {
            await info.beforeNext();
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.SETUP_ACCESS_MODAL);
            isCreating.value = false;
            return;
        }
    }

    if (info.next.value) {
        step.value = info.next.value;
    } else {
        model.value = false;
    }
}

/**
 * Navigates to the previous step.
 */
function prevStep(): void {
    const info = stepInfos[step.value];

    info.ref.value?.onExit?.('prev');

    if (info.prev.value) {
        step.value = info.prev.value;
    } else {
        model.value = false;
    }
}

function sendApplicationsAnalytics(e: AnalyticsEvent): void {
    if (selectedApp.value) analyticsStore.eventTriggered(e, { application: selectedApp.value.name });
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
 * Executes when the dialog's inner content has been added or removed.
 * If removed, refs are reset back to their initial values.
 * Otherwise, data is fetched and the current step is initialized.
 *
 * This is used instead of onMounted because the dialog remains mounted
 * even when hidden.
 */
watch(innerContent, async (comp?: VCard): Promise<void> => {
    if (!comp) {
        resets.forEach(reset => reset());
        return;
    }

    isFetching.value = true;

    const projectID = projectsStore.state.selectedProject.id;

    await Promise.all([
        agStore.getAllAGNames(projectID),
        bucketsStore.getAllBucketsNames(projectID),
    ]).catch(error => {
        notify.notifyError(error, AnalyticsErrorEventSource.SETUP_ACCESS_MODAL);
    });

    passphrase.value = bucketsStore.state.passphrase;
    setDefaultName();

    isFetching.value = false;

    stepInfos[step.value].ref.value?.onEnter?.();
});

watch(model, value => {
    if (!value && step.value === SetupStep.AccessCreatedStep) emit('accessCreated');
});
</script>

<style scoped lang="scss">
.setup-app__window {
    transition: opacity 250ms cubic-bezier(0.4, 0, 0.2, 1);

    &--loading {
        opacity: 0.3;
        transition: opacity 0s;
        pointer-events: none;
    }
}
</style>

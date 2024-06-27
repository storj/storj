// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        max-width="450px"
        transition="fade-transition"
    >
        <v-card ref="innerContent">
            <v-sheet>
                <v-card-item class="pa-6">
                    <v-card-title class="font-weight-bold">
                        New {{ app ? app.name : '' }} Access
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
                    <template v-if="app" #prepend>
                        <img :src="app.src" :alt="app.name" width="40" height="40" class="rounded">
                    </template>
                    <template v-else #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <IconAccess />
                        </v-sheet>
                    </template>
                    <v-progress-linear height="2px" indeterminate absolute location="bottom" :active="isFetching || isCreating" />
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-window
                v-model="step"
                class="setup-app__window"
                :class="{ 'setup-app__window--loading': isFetching }"
            >
                <v-window-item :value="SetupStep.ChooseAccessStep">
                    <choose-access-step
                        :ref="stepInfos[SetupStep.ChooseAccessStep].ref"
                        @name-changed="newName => name = newName"
                        @typeChanged="newType => accessType = newType"
                        @submit="nextStep"
                    />
                </v-window-item>

                <v-window-item :value="SetupStep.EncryptionInfo">
                    <encryption-info-step :ref="stepInfos[SetupStep.EncryptionInfo].ref" />
                </v-window-item>

                <v-window-item :value="SetupStep.ChooseFlowStep">
                    <choose-flow-step
                        :ref="stepInfos[SetupStep.ChooseFlowStep].ref"
                        :app="app"
                        @setFlowType="val => flowType = val"
                    />
                </v-window-item>

                <v-window-item :value="SetupStep.AccessEncryption">
                    <access-encryption-step
                        :ref="stepInfos[SetupStep.AccessEncryption].ref"
                        @selectOption="val => passphraseOption = val"
                        @passphraseChanged="val => passphrase = val"
                        @submit="nextStep"
                    />
                </v-window-item>

                <v-window-item :value="SetupStep.EnterNewPassphrase">
                    <enter-passphrase-step
                        :ref="stepInfos[SetupStep.EnterNewPassphrase].ref"
                        @passphraseChanged="val => passphrase = val"
                    />
                </v-window-item>

                <v-window-item :value="SetupStep.PassphraseGenerated">
                    <passphrase-generated-step
                        :ref="stepInfos[SetupStep.PassphraseGenerated].ref"
                        :name="name"
                        @passphraseChanged="val => passphrase = val"
                    />
                </v-window-item>

                <v-window-item :value="SetupStep.ChoosePermissionsStep">
                    <choose-permissions-step
                        :ref="stepInfos[SetupStep.ChoosePermissionsStep].ref"
                        @permissionsChanged="val => permissions = val"
                    />
                </v-window-item>

                <v-window-item :value="SetupStep.SelectBucketsStep">
                    <select-buckets-step
                        :ref="stepInfos[SetupStep.SelectBucketsStep].ref"
                        @bucketsChanged="val => buckets = val"
                    />
                </v-window-item>

                <v-window-item :value="SetupStep.OptionalExpirationStep">
                    <optional-expiration-step
                        :ref="stepInfos[SetupStep.OptionalExpirationStep].ref"
                        :end-date="endDate"
                        @endDateChanged="val => endDate = val"
                    />
                </v-window-item>

                <v-window-item :value="SetupStep.ConfirmDetailsStep">
                    <confirm-details-step
                        :ref="stepInfos[SetupStep.ConfirmDetailsStep].ref"
                        :name="name"
                        :type="accessType"
                        :permissions="permissions"
                        :buckets="buckets"
                        :end-date="endDate"
                    />
                </v-window-item>

                <v-window-item :value="SetupStep.AccessCreatedStep">
                    <access-created-step
                        :ref="stepInfos[SetupStep.AccessCreatedStep].ref"
                        :name="name"
                        :app="app"
                        :cli-access="cliAccess"
                        :access-grant="accessGrant"
                        :credentials="credentials"
                        :access-type="accessType"
                    />
                </v-window-item>
            </v-window>

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
                            :href="docsLink"
                            target="_blank"
                            rel="noopener noreferrer"
                            @click="() => sendApplicationsAnalytics(AnalyticsEvent.APPLICATIONS_DOCS_CLICKED)"
                        >
                            <template #prepend>
                                <IconDocs />
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
import { Component, computed, Ref, ref, watch, WatchStopHandle } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
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

import IconAccess from '../icons/IconAccess.vue';

import {
    AccessType,
    FlowType,
    PassphraseOption,
    Permission,
    SetupStep,
} from '@/types/createAccessGrant';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { getUniqueName, IDialogFlowStep } from '@/types/common';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { ROUTES } from '@/router';
import { useUsersStore } from '@/store/modules/usersStore';
import { Application } from '@/types/applications';

import ChooseFlowStep from '@/components/dialogs/accessSetupSteps/ChooseFlowStep.vue';
import ChooseAccessStep from '@/components/dialogs/accessSetupSteps/ChooseAccessStep.vue';
import ChoosePermissionsStep from '@/components/dialogs/accessSetupSteps/ChoosePermissionsStep.vue';
import SelectBucketsStep from '@/components/dialogs/accessSetupSteps/SelectBucketsStep.vue';
import AccessCreatedStep from '@/components/dialogs/accessSetupSteps/AccessCreatedStep.vue';
import AccessEncryptionStep from '@/components/dialogs/createAccessSteps/AccessEncryptionStep.vue';
import EnterPassphraseStep from '@/components/dialogs/commonPassphraseSteps/EnterPassphraseStep.vue';
import PassphraseGeneratedStep from '@/components/dialogs/commonPassphraseSteps/PassphraseGeneratedStep.vue';
import OptionalExpirationStep from '@/components/dialogs/accessSetupSteps/OptionalExpirationStep.vue';
import EncryptionInfoStep from '@/components/dialogs/createAccessSteps/EncryptionInfoStep.vue';
import ConfirmDetailsStep from '@/components/dialogs/accessSetupSteps/ConfirmDetailsStep.vue';
import IconDocs from '@/components/icons/IconDocs.vue';

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
        public beforeNext?: () => Promise<void>,
    ) {
        this.prev = (typeof prev === 'function') ? computed<SetupStep | undefined>(prev) : ref<SetupStep | undefined>(prev);
        this.next = (typeof next === 'function') ? computed<SetupStep | undefined>(next) : ref<SetupStep | undefined>(next);
        this.nextText = (typeof nextText === 'function') ? computed<string>(nextText) : ref<string>(nextText);
        this.prevText = (typeof prevText === 'function') ? computed<string>(prevText) : ref<string>(prevText);
    }
}

const props = withDefaults(defineProps<{
    docsLink: string
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

const model = defineModel<boolean>({ required: true });

const innerContent = ref<Component>();
const isCreating = ref<boolean>(false);
const isFetching = ref<boolean>(true);
const worker = ref<Worker | null>(null);

const resets: (() => void)[] = [];
function resettableRef<T>(value: T): Ref<T> {
    const thisRef = ref<T>(value) as Ref<T>;
    resets.push(() => thisRef.value = value);
    return thisRef;
}

const step = resettableRef<SetupStep>(props.defaultAccessType ? SetupStep.ChooseFlowStep : SetupStep.ChooseAccessStep);
const accessType = resettableRef<AccessType>(props.defaultAccessType ?? AccessType.S3);
const flowType = resettableRef<FlowType>(FlowType.FullAccess);
const name = resettableRef<string>('');
const permissions = resettableRef<Permission[]>([]);
const buckets = resettableRef<string[]>([]);
const passphrase = resettableRef<string>(bucketsStore.state.passphrase);
const endDate = resettableRef<Date | null>(null);
const passphraseOption = resettableRef<PassphraseOption>(PassphraseOption.EnterNewPassphrase);
const cliAccess = resettableRef<string>('');
const accessGrant = resettableRef<string>('');
const credentials = resettableRef<EdgeCredentials>(new EdgeCredentials());

const promptForPassphrase = computed<boolean>(() => bucketsStore.state.promptForPassphrase);

const stepInfos: Record<SetupStep, StepInfo> = {
    [SetupStep.ChooseAccessStep]: new StepInfo(
        'Next ->',
        'Cancel',
        undefined,
        () => (accessType.value === AccessType.S3 && !userStore.noticeDismissal.serverSideEncryption)
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
        () => {
            if (accessType.value === AccessType.APIKey) {
                return flowType.value === FlowType.FullAccess ? 'Create Access' : 'Next ->';
            }

            return promptForPassphrase.value || flowType.value === FlowType.Advanced ? 'Next ->' : 'Create Access';
        },
        () => props.defaultAccessType ? 'Cancel' : 'Back',
        () => {
            if (props.defaultAccessType) return undefined;

            return accessType.value === AccessType.S3 && !userStore.noticeDismissal.serverSideEncryption
                ? SetupStep.EncryptionInfo
                : SetupStep.ChooseAccessStep;
        },
        () => {
            if (accessType.value === AccessType.APIKey) {
                return flowType.value === FlowType.FullAccess ? SetupStep.AccessCreatedStep : SetupStep.ChoosePermissionsStep;
            }

            if (promptForPassphrase.value) return SetupStep.AccessEncryption;

            return flowType.value === FlowType.FullAccess ? SetupStep.AccessCreatedStep : SetupStep.ChoosePermissionsStep;
        },
        async () => {
            if (flowType.value === FlowType.FullAccess && (accessType.value === AccessType.APIKey || !promptForPassphrase.value)) {
                await generate();
            }
        },
    ),
    [SetupStep.AccessEncryption]: new StepInfo(
        () => flowType.value === FlowType.FullAccess && passphraseOption.value === PassphraseOption.SetMyProjectPassphrase ? 'Create Access' : 'Next ->',
        'Back',
        SetupStep.ChooseFlowStep,
        () => {
            if (passphraseOption.value === PassphraseOption.EnterNewPassphrase) return SetupStep.EnterNewPassphrase;
            if (passphraseOption.value === PassphraseOption.GenerateNewPassphrase) return SetupStep.PassphraseGenerated;

            return flowType.value === FlowType.FullAccess ? SetupStep.AccessCreatedStep : SetupStep.ChoosePermissionsStep;
        },
        async () => {
            if (
                passphraseOption.value === PassphraseOption.EnterNewPassphrase ||
                passphraseOption.value === PassphraseOption.GenerateNewPassphrase ||
                flowType.value === FlowType.Advanced
            ) return;

            await generate();
        },
    ),
    [SetupStep.PassphraseGenerated]: new StepInfo(
        () => flowType.value === FlowType.FullAccess ? 'Create Access' : 'Next ->',
        'Back',
        SetupStep.AccessEncryption,
        () => flowType.value === FlowType.FullAccess ? SetupStep.AccessCreatedStep : SetupStep.ChoosePermissionsStep,
        async () => {
            if (flowType.value === FlowType.FullAccess) await generate();
        },
    ),
    [SetupStep.EnterNewPassphrase]: new StepInfo(
        () => flowType.value === FlowType.FullAccess ? 'Create Access' : 'Next ->',
        'Back',
        SetupStep.AccessEncryption,
        () => flowType.value === FlowType.FullAccess ? SetupStep.AccessCreatedStep : SetupStep.ChoosePermissionsStep,
        async () => {
            if (flowType.value === FlowType.FullAccess) await generate();
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
        SetupStep.SelectBucketsStep,
    ),
    [SetupStep.SelectBucketsStep]: new StepInfo(
        'Next ->',
        'Back',
        SetupStep.ChoosePermissionsStep,
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
        SetupStep.OptionalExpirationStep,
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
        if (passphraseOption.value === PassphraseOption.SetMyProjectPassphrase) {
            bucketsStore.setEdgeCredentials(new EdgeCredentials());
            bucketsStore.setPassphrase(passphrase.value);
            bucketsStore.setPromptForPassphrase(false);
        }
    }
    if (accessType.value === AccessType.S3) await createEdgeCredentials();

    sendApplicationsAnalytics(AnalyticsEvent.APPLICATIONS_SETUP_COMPLETED);

    isCreating.value = false;
}

/**
 * Generates API Key.
 */
async function createAPIKey(): Promise<void> {
    if (!worker.value) throw new Error('Web worker is not initialized.');

    const projectID = projectsStore.state.selectedProject.id;
    const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(name.value, projectID);

    if (route.name === ROUTES.Access.name) {
        agStore.getAccessGrants(1, projectID).catch(err => {
            notify.error(`Unable to fetch access grants. ${err.message}`, AnalyticsErrorEventSource.SETUP_ACCESS_MODAL);
        });
    }

    const noCaveats = flowType.value === FlowType.FullAccess;

    let permissionsMsg = {
        'type': 'SetPermission',
        'buckets': JSON.stringify(noCaveats ? [] : buckets.value),
        'apiKey': cleanAPIKey.secret,
        'isDownload': noCaveats || permissions.value.includes(Permission.Read),
        'isUpload': noCaveats || permissions.value.includes(Permission.Write),
        'isList': noCaveats || permissions.value.includes(Permission.List),
        'isDelete': noCaveats || permissions.value.includes(Permission.Delete),
        'notBefore': new Date().toISOString(),
    };

    if (endDate.value && !noCaveats) permissionsMsg = Object.assign(permissionsMsg, { 'notAfter': endDate.value.toISOString() });

    worker.value.postMessage(permissionsMsg);

    const grantEvent: MessageEvent = await new Promise(resolve => {
        if (worker.value) worker.value.onmessage = resolve;
    });
    if (grantEvent.data.error) throw new Error(grantEvent.data.error);

    cliAccess.value = grantEvent.data.value;

    if (accessType.value === AccessType.APIKey) analyticsStore.eventTriggered(AnalyticsEvent.API_ACCESS_CREATED);
}

/**
 * Generates access grant.
 */
async function createAccessGrant(): Promise<void> {
    if (!worker.value) throw new Error('Web worker is not initialized.');
    if (!passphrase.value) throw new Error('Passphrase can\'t be empty');

    const satelliteNodeURL = configStore.state.config.satelliteNodeURL;

    const salt = await projectsStore.getProjectSalt(projectsStore.state.selectedProject.id);

    worker.value.postMessage({
        'type': 'GenerateAccess',
        'apiKey': cliAccess.value,
        'passphrase': passphrase.value,
        'salt': salt,
        'satelliteNodeURL': satelliteNodeURL,
    });

    const accessEvent: MessageEvent = await new Promise(resolve => {
        if (worker.value) worker.value.onmessage = resolve;
    });
    if (accessEvent.data.error) throw new Error(accessEvent.data.error);

    accessGrant.value = accessEvent.data.value;

    if (accessType.value === AccessType.AccessGrant) analyticsStore.eventTriggered(AnalyticsEvent.ACCESS_GRANT_CREATED);
}

/**
 * Generates edge credentials.
 */
async function createEdgeCredentials(): Promise<void> {
    credentials.value = await agStore.getEdgeCredentials(accessGrant.value);
    analyticsStore.eventTriggered(AnalyticsEvent.GATEWAY_CREDENTIALS_CREATED);
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
    if (props.app) analyticsStore.eventTriggered(e, { application: props.app.name });
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
watch(innerContent, async (comp: Component): Promise<void> => {
    if (!comp) {
        resets.forEach(reset => reset());
        return;
    }

    worker.value = agStore.state.accessGrantsWebWorker;
    if (worker.value) {
        worker.value.onerror = (error: ErrorEvent) => {
            notify.error(error.message, AnalyticsErrorEventSource.SETUP_ACCESS_MODAL);
        };
    }

    isFetching.value = true;

    const projectID = projectsStore.state.selectedProject.id;
    await agStore.getAllAGNames(projectID).catch(err => {
        notify.error(`Error fetching access grant names. ${err.message}`, AnalyticsErrorEventSource.SETUP_ACCESS_MODAL);
    });
    await bucketsStore.getAllBucketsNames(projectID).catch(err => {
        notify.error(`Error fetching bucket grant names. ${err.message}`, AnalyticsErrorEventSource.SETUP_ACCESS_MODAL);
    });

    passphrase.value = bucketsStore.state.passphrase;
    setDefaultName();

    isFetching.value = false;

    stepInfos[step.value].ref.value?.onEnter?.();
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

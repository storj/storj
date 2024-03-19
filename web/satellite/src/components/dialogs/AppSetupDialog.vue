// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        min-width="400px"
        max-width="450px"
        transition="fade-transition"
    >
        <v-card ref="innerContent" rounded="xlg">
            <v-sheet>
                <v-card-item class="py-4 pl-7">
                    <v-card-title class="font-weight-bold">
                        Setup App Access
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
            </v-sheet>

            <v-divider />

            <v-window
                v-model="step"
                class="setup-app__window"
                :class="{ 'setup-app__window--loading': isFetching }"
            >
                <v-window-item :value="SetupStep.ChooseFlowStep">
                    <choose-flow-step
                        :ref="stepInfos[SetupStep.ChooseFlowStep].ref"
                        @setFlowType="val => flowType = val"
                    />
                </v-window-item>

                <v-window-item :value="SetupStep.AccessEncryption">
                    <access-encryption-step
                        :ref="stepInfos[SetupStep.AccessEncryption].ref"
                        @selectOption="val => passphraseOption = val"
                        @passphraseChanged="val => passphrase = val"
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

                <v-window-item :value="SetupStep.AccessCreatedStep">
                    <access-created-step
                        :ref="stepInfos[SetupStep.AccessCreatedStep].ref"
                        :credentials="credentials"
                    />
                </v-window-item>
            </v-window>

            <v-divider />

            <v-card-actions class="pa-7">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            :disabled="isCreating || isFetching"
                            @click="prevStep"
                        >
                            {{ stepInfos[step].prevText }}
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
                        >
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

import {
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

import ChooseFlowStep from '@/components/dialogs/appSetupSteps/ChooseFlowStep.vue';
import ChoosePermissionsStep from '@/components/dialogs/appSetupSteps/ChoosePermissionsStep.vue';
import SelectBucketsStep from '@/components/dialogs/appSetupSteps/SelectBucketsStep.vue';
import AccessCreatedStep from '@/components/dialogs/appSetupSteps/AccessCreatedStep.vue';
import AccessEncryptionStep from '@/components/dialogs/createAccessSteps/AccessEncryptionStep.vue';
import EnterPassphraseStep from '@/components/dialogs/commonPassphraseSteps/EnterPassphraseStep.vue';
import PassphraseGeneratedStep from '@/components/dialogs/commonPassphraseSteps/PassphraseGeneratedStep.vue';

type SetupLocation = SetupStep | undefined | (() => (SetupStep | undefined));

class StepInfo {
    public ref = ref<IDialogFlowStep>();
    public prev: Ref<SetupStep | undefined>;
    public next: Ref<SetupStep | undefined>;
    public nextText: Ref<string>;

    constructor(
        nextText: string | (() => string),
        public prevText: string,
        prev: SetupLocation = undefined,
        next: SetupLocation = undefined,
        public beforeNext?: () => Promise<void>,
    ) {
        this.prev = (typeof prev === 'function') ? computed<SetupStep | undefined>(prev) : ref<SetupStep | undefined>(prev);
        this.next = (typeof next === 'function') ? computed<SetupStep | undefined>(next) : ref<SetupStep | undefined>(next);
        this.nextText = (typeof nextText === 'function') ? computed<string>(nextText) : ref<string>(nextText);
    }
}

const props = defineProps<{
    accessName: string
    docsLink: string
}>();

const agStore = useAccessGrantsStore();
const bucketsStore = useBucketsStore();
const projectsStore = useProjectsStore();
const configStore = useConfigStore();
const analyticsStore = useAnalyticsStore();

const notify = useNotify();

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

const step = resettableRef<SetupStep>(SetupStep.ChooseFlowStep);
const flowType = resettableRef<FlowType>(FlowType.Default);
const name = resettableRef<string>('');
const permissions = resettableRef<Permission[]>([]);
const buckets = resettableRef<string[]>([]);
const passphrase = resettableRef<string>(bucketsStore.state.passphrase);
const passphraseOption = resettableRef<PassphraseOption>(PassphraseOption.EnterNewPassphrase);
const credentials = resettableRef<EdgeCredentials>(new EdgeCredentials());

const promptForPassphrase = computed<boolean>(() => bucketsStore.state.promptForPassphrase);

const stepInfos: Record<SetupStep, StepInfo> = {
    [SetupStep.ChooseFlowStep]: new StepInfo(
        () => promptForPassphrase.value || flowType.value === FlowType.Advanced ? 'Next' : 'Create Access',
        'Cancel',
        undefined,
        () => {
            if (promptForPassphrase.value) return SetupStep.AccessEncryption;

            return flowType.value === FlowType.Default ? SetupStep.AccessCreatedStep : SetupStep.ChoosePermissionsStep;
        },
        async () => {
            if (!promptForPassphrase.value && flowType.value === FlowType.Default) {
                await createCredentials();
            }
        },
    ),
    [SetupStep.AccessEncryption]: new StepInfo(
        () => flowType.value === FlowType.Default && passphraseOption.value === PassphraseOption.SetMyProjectPassphrase ? 'Create Access' : 'Next',
        'Back',
        SetupStep.ChooseFlowStep,
        () => {
            if (passphraseOption.value === PassphraseOption.EnterNewPassphrase) return SetupStep.EnterNewPassphrase;
            if (passphraseOption.value === PassphraseOption.GenerateNewPassphrase) return SetupStep.PassphraseGenerated;

            return flowType.value === FlowType.Default ? SetupStep.AccessCreatedStep : SetupStep.ChoosePermissionsStep;
        },
        async () => {
            if (
                passphraseOption.value === PassphraseOption.EnterNewPassphrase ||
                passphraseOption.value === PassphraseOption.GenerateNewPassphrase ||
                flowType.value === FlowType.Advanced
            ) return;

            await createCredentials();
        },
    ),
    [SetupStep.PassphraseGenerated]: new StepInfo(
        () => flowType.value === FlowType.Default ? 'Create Access' : 'Next',
        'Back',
        SetupStep.AccessEncryption,
        () => flowType.value === FlowType.Default ? SetupStep.AccessCreatedStep : SetupStep.ChoosePermissionsStep,
        async () => {
            if (flowType.value === FlowType.Default) await createCredentials();
        },
    ),
    [SetupStep.EnterNewPassphrase]: new StepInfo(
        () => flowType.value === FlowType.Default ? 'Create Access' : 'Next',
        'Back',
        SetupStep.AccessEncryption,
        () => flowType.value === FlowType.Default ? SetupStep.AccessCreatedStep : SetupStep.ChoosePermissionsStep,
        async () => {
            if (flowType.value === FlowType.Default) await createCredentials();
        },
    ),
    [SetupStep.ChoosePermissionsStep]: new StepInfo(
        'Next',
        'Back',
        () => {
            if (bucketsStore.state.promptForPassphrase) {
                if (passphraseOption.value === PassphraseOption.EnterNewPassphrase) return SetupStep.EnterNewPassphrase;
                if (passphraseOption.value === PassphraseOption.GenerateNewPassphrase) return SetupStep.PassphraseGenerated;

                return SetupStep.AccessEncryption;
            }

            return SetupStep.ChooseFlowStep;
        },
        SetupStep.SelectBucketsStep,
    ),
    [SetupStep.SelectBucketsStep]: new StepInfo(
        'Create Access',
        'Back',
        SetupStep.ChoosePermissionsStep,
        SetupStep.AccessCreatedStep,
        createCredentials,
    ),
    [SetupStep.AccessCreatedStep]: new StepInfo('', 'Close'),
};

/**
 * Set unique name of access to be created.
 */
function setDefaultName(): void {
    name.value = getUniqueName(props.accessName, agStore.state.allAGNames);
}

/**
 * Set unique name of access to be created.
 */
async function createCredentials(): Promise<void> {
    if (!worker.value) {
        throw new Error('Web worker is not initialized.');
    }

    if (!passphrase.value) throw new Error('Passphrase can\'t be empty');

    const projectID = projectsStore.state.selectedProject.id;

    isCreating.value = true;

    const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(name.value, projectID);

    const noCaveats = flowType.value === FlowType.Default;

    const permissionsMsg = {
        'type': 'SetPermission',
        'buckets': JSON.stringify(noCaveats ? [] : buckets.value),
        'apiKey': cleanAPIKey.secret,
        'isDownload': noCaveats || permissions.value.includes(Permission.Read),
        'isUpload': noCaveats || permissions.value.includes(Permission.Write),
        'isList': noCaveats || permissions.value.includes(Permission.List),
        'isDelete': noCaveats || permissions.value.includes(Permission.Delete),
        'notBefore': new Date().toISOString(),
    };

    worker.value.postMessage(permissionsMsg);

    const grantEvent: MessageEvent = await new Promise(resolve => {
        if (worker.value) worker.value.onmessage = resolve;
    });
    if (grantEvent.data.error) throw new Error(grantEvent.data.error);

    const keyWithCaveats = grantEvent.data.value;
    const satelliteNodeURL = configStore.state.config.satelliteNodeURL;

    const salt = await projectsStore.getProjectSalt(projectsStore.state.selectedProject.id);

    worker.value.postMessage({
        'type': 'GenerateAccess',
        'apiKey': keyWithCaveats,
        'passphrase': passphrase.value,
        'salt': salt,
        'satelliteNodeURL': satelliteNodeURL,
    });

    const accessEvent: MessageEvent = await new Promise(resolve => {
        if (worker.value) worker.value.onmessage = resolve;
    });
    if (accessEvent.data.error) throw new Error(accessEvent.data.error);

    const accessGrant = accessEvent.data.value;

    credentials.value = await agStore.getEdgeCredentials(accessGrant);
    analyticsStore.eventTriggered(AnalyticsEvent.GATEWAY_CREDENTIALS_CREATED);

    if (passphraseOption.value === PassphraseOption.SetMyProjectPassphrase) {
        bucketsStore.setEdgeCredentials(new EdgeCredentials());
        bucketsStore.setPassphrase(passphrase.value);
        bucketsStore.setPromptForPassphrase(false);
    }

    isCreating.value = false;
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
            notify.notifyError(error, AnalyticsErrorEventSource.SETUP_APPLICATION_MODAL);
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

    passphrase.value = bucketsStore.state.passphrase;
    if (props.accessName) setDefaultName();

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

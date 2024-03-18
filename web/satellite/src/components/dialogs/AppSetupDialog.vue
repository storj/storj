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
                            @click="model = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-window v-model="step">
                <v-window-item :value="SetupStep.ChooseFlowStep">
                    <ChooseFlowStep @setFlowType="val => flowType = val" />
                </v-window-item>

                <v-window-item :value="SetupStep.AccessEncryption">
                    <AccessEncryptionStep @passphraseChanged="val => passphrase = val" />
                </v-window-item>

                <v-window-item :value="SetupStep.ChoosePermissionsStep">
                    <ChoosePermissionsStep />
                </v-window-item>

                <v-window-item :value="SetupStep.SelectBucketsStep">
                    <SelectBucketsStep />
                </v-window-item>

                <v-window-item :value="SetupStep.AccessCreatedStep">
                    <AccessCreatedStep />
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
                            @click="prevStep"
                        >
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            v-if="step !== SetupStep.AccessCreatedStep"
                            color="primary"
                            variant="flat"
                            block
                            @click="nextStep"
                        >
                            Next
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
import { Component, computed, Ref, ref, watch } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VCol,
    VDialog,
    VDivider,
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
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

import ChooseFlowStep from '@/components/dialogs/appSetupSteps/ChooseFlowStep.vue';
import ChoosePermissionsStep from '@/components/dialogs/appSetupSteps/ChoosePermissionsStep.vue';
import SelectBucketsStep from '@/components/dialogs/appSetupSteps/SelectBucketsStep.vue';
import AccessCreatedStep from '@/components/dialogs/appSetupSteps/AccessCreatedStep.vue';
import AccessEncryptionStep from '@/components/dialogs/createAccessSteps/AccessEncryptionStep.vue';

class StepInfo {
    public ref: Ref<IDialogFlowStep | undefined> = ref<IDialogFlowStep>();

    constructor(
        public nextText: string,
        public prev?: SetupStep,
        public next?: SetupStep,
        public beforeNext?: () => Promise<void>,
    ) { }
}

const props = defineProps<{
    accessName: string
    docsLink: string
}>();

const agStore = useAccessGrantsStore();
const bucketsStore = useBucketsStore();
const projectsStore = useProjectsStore();

const notify = useNotify();

const resets: (() => void)[] = [];

const model = defineModel<boolean>({ required: true });

const innerContent = ref<Component>();
const isCreating = ref<boolean>(false);
const isFetching = ref<boolean>(true);

function resettableRef<T>(value: T): Ref<T> {
    const thisRef = ref<T>(value) as Ref<T>;
    resets.push(() => thisRef.value = value);
    return thisRef;
}

const worker = ref<Worker | null>(null);
const step = resettableRef<SetupStep>(SetupStep.ChooseFlowStep);
const flowType = resettableRef<FlowType>(FlowType.Default);
const name = resettableRef<string>('');
const permissions = resettableRef<Permission[]>([]);
const buckets = resettableRef<string[]>([]);
const passphrase = resettableRef<string>(bucketsStore.state.passphrase);
const passphraseOption = resettableRef<PassphraseOption>(PassphraseOption.EnterNewPassphrase);

const promptForPassphrase = computed<boolean>(() => bucketsStore.state.promptForPassphrase);

const stepInfos: Record<SetupStep, StepInfo> = {
    [SetupStep.ChooseFlowStep]: new StepInfo(
        promptForPassphrase.value || flowType.value === FlowType.Advanced ? 'Next' : 'Create Access',
        undefined,
        bucketsStore.state.promptForPassphrase
            ? SetupStep.AccessEncryption
            : flowType.value === FlowType.Default
                ? SetupStep.AccessCreatedStep
                : SetupStep.ChoosePermissionsStep,
        !promptForPassphrase.value && flowType.value === FlowType.Default ? createCredentials : undefined,
    ),
    [SetupStep.AccessEncryption]: new StepInfo(
        'Next',
        SetupStep.ChooseFlowStep,
        flowType.value === FlowType.Default
            ? SetupStep.AccessCreatedStep
            : passphraseOption.value === PassphraseOption.EnterNewPassphrase
                ? SetupStep.EnterNewPassphrase
                : SetupStep.PassphraseGenerated,
        flowType.value === FlowType.Default ? createCredentials : undefined,
    ),
    [SetupStep.ChoosePermissionsStep]: new StepInfo(''),
    [SetupStep.PassphraseGenerated]: new StepInfo(''),
    [SetupStep.EnterMyPassphrase]: new StepInfo(''),
    [SetupStep.EnterNewPassphrase]: new StepInfo(''),
    [SetupStep.SelectBucketsStep]: new StepInfo(''),
    [SetupStep.AccessCreatedStep]: new StepInfo(''),
};

/**
 * Set unique name of access to be created.
 */
function setDefaultName(): void {
    if (props.accessName) {
        name.value = getUniqueName(props.accessName, agStore.state.allAGNames);
    }
}

/**
 * Set unique name of access to be created.
 */
async function createCredentials(): Promise<void> {}

/**
 * Navigates to the next step.
 */
async function nextStep(): Promise<void> {
    const info = stepInfos[step.value];

    if (isCreating.value || isFetching.value || info.ref.value?.validate?.() === false) return;

    if (info.beforeNext) {
        try {
            await info.beforeNext();
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.CREATE_AG_MODAL);
            return;
        }
    }

    info.ref.value?.onExit?.('next');

    if (info.next) {
        step.value = info.next;
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

    if (info.prev) {
        step.value = info.prev;
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
    let unwatchImmediately = false;
    const unwatch = watch(
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

    if (props.accessName) {
        setDefaultName();
    }

    isFetching.value = false;

    stepInfos[step.value].ref.value?.onEnter?.();
});
</script>

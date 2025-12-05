// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        min-width="320px"
        max-width="400px"
        transition="fade-transition"
        scrollable
        persistent
    >
        <v-card ref="innerContent" rounded="xlg">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <v-icon :icon="Globe" size="18" />
                        </v-sheet>
                    </template>

                    <v-card-title class="font-weight-bold">
                        {{ currentTitle }}
                    </v-card-title>

                    <template #append>
                        <v-btn
                            :icon="X"
                            variant="text"
                            size="small"
                            color="default"
                            @click="model = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-card-text class="pa-0">
                <v-window
                    v-model="step"
                    :touch="false"
                    class="new-domain__window"
                    :class="{ 'new-domain__window--loading': isFetching || isGenerating }"
                >
                    <v-window-item :value="NewDomainFlowStep.CustomDomain">
                        <new-custom-domain-step
                            :ref="stepInfos[NewDomainFlowStep.CustomDomain].ref"
                            @domain-changed="val => domain = val"
                            @bucket-changed="val => bucket = val"
                            @submit="nextStep"
                        />
                    </v-window-item>

                    <v-window-item :value="NewDomainFlowStep.SetupDomainAccess">
                        <access-encryption-step
                            :ref="stepInfos[NewDomainFlowStep.SetupDomainAccess].ref"
                            @select-option="val => passphraseOption = val"
                            @passphrase-changed="val => passphrase = val"
                            @submit="nextStep"
                        />
                    </v-window-item>

                    <v-window-item :value="NewDomainFlowStep.EnterNewPassphrase">
                        <enter-passphrase-step
                            :ref="stepInfos[NewDomainFlowStep.EnterNewPassphrase].ref"
                            @passphrase-changed="val => passphrase = val"
                        />
                    </v-window-item>

                    <v-window-item :value="NewDomainFlowStep.PassphraseGenerated">
                        <passphrase-generated-step
                            :ref="stepInfos[NewDomainFlowStep.PassphraseGenerated].ref"
                            :name="accessName"
                            @passphrase-changed="val => passphrase = val"
                        />
                    </v-window-item>

                    <v-window-item :value="NewDomainFlowStep.SetupCNAME">
                        <setup-c-n-a-m-e-step
                            :ref="stepInfos[NewDomainFlowStep.SetupCNAME].ref"
                            :domain="domain"
                            :cname="cname"
                        />
                    </v-window-item>

                    <v-window-item :value="NewDomainFlowStep.SetupTXT">
                        <setup-t-x-t-step
                            :ref="stepInfos[NewDomainFlowStep.SetupTXT].ref"
                            :domain="domain"
                            :storj-root="storjRoot"
                            :storj-access="storjAccess"
                            :storj-tls="storjTLS"
                        />
                    </v-window-item>

                    <v-window-item :value="NewDomainFlowStep.VerifyDomain">
                        <verify-domain-step
                            :ref="stepInfos[NewDomainFlowStep.VerifyDomain].ref"
                            :domain="domain"
                            :cname="cname"
                            :txt="txt"
                        />
                    </v-window-item>

                    <v-window-item :value="NewDomainFlowStep.DomainConnected">
                        <domain-connected-step
                            :ref="stepInfos[NewDomainFlowStep.DomainConnected].ref"
                        />
                    </v-window-item>
                </v-window>
            </v-card-text>

            <v-divider />

            <v-card-actions class="pa-7">
                <v-row>
                    <v-col v-if="step === NewDomainFlowStep.CustomDomain">
                        <v-btn
                            variant="outlined"
                            color="default"
                            href="https://docs.storj.io/dcs/code/static-site-hosting/custom-domains"
                            target="_blank"
                            rel="noopener noreferrer"
                            block
                        >
                            Learn More
                        </v-btn>
                    </v-col>
                    <v-col v-else-if="stepInfos[step].prev.value">
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            @click="prevStep"
                        >
                            Back
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary"
                            variant="flat"
                            block
                            :loading="isFetching || isGenerating"
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
import { Component, computed, Ref, ref, watch, WatchStopHandle } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VCardText,
    VCol,
    VDialog,
    VDivider,
    VRow,
    VSheet,
    VWindow,
    VWindowItem,
    VIcon,
} from 'vuetify/components';
import { Globe, X } from 'lucide-vue-next';

import { NewDomainFlowStep } from '@/types/domains';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useDomainsStore } from '@/store/modules/domainsStore';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { IDialogFlowStep } from '@/types/common';
import { PassphraseOption } from '@/types/setupAccess';
import { useLinksharing } from '@/composables/useLinksharing';

import AccessEncryptionStep from '@/components/dialogs/accessSetupSteps/AccessEncryptionStep.vue';
import EnterPassphraseStep from '@/components/dialogs/commonPassphraseSteps/EnterPassphraseStep.vue';
import PassphraseGeneratedStep from '@/components/dialogs/commonPassphraseSteps/PassphraseGeneratedStep.vue';
import NewCustomDomainStep from '@/components/dialogs/newDomainSteps/NewCustomDomainStep.vue';
import SetupCNAMEStep from '@/components/dialogs/newDomainSteps/SetupCNAMEStep.vue';
import SetupTXTStep from '@/components/dialogs/newDomainSteps/SetupTXTStep.vue';
import VerifyDomainStep from '@/components/dialogs/newDomainSteps/VerifyDomainStep.vue';
import DomainConnectedStep from '@/components/dialogs/newDomainSteps/DomainConnectedStep.vue';

type FlowLocation = NewDomainFlowStep | undefined | (() => (NewDomainFlowStep | undefined));

class StepInfo {
    public ref = ref<IDialogFlowStep>();
    public prev: Ref<NewDomainFlowStep | undefined>;
    public next: Ref<NewDomainFlowStep | undefined>;
    public nextText: Ref<string>;

    constructor(
        nextText: string | (() => string),
        prev: FlowLocation = undefined,
        next: FlowLocation = undefined,
        public beforeNext?: () => Promise<void>,
    ) {
        this.prev = (typeof prev === 'function') ? computed<NewDomainFlowStep | undefined>(prev) : ref<NewDomainFlowStep | undefined>(prev);
        this.next = (typeof next === 'function') ? computed<NewDomainFlowStep | undefined>(next) : ref<NewDomainFlowStep | undefined>(next);
        this.nextText = (typeof nextText === 'function') ? computed<string>(nextText) : ref<string>(nextText);
    }
}

const bucketsStore = useBucketsStore();
const projectsStore = useProjectsStore();
const domainsStore = useDomainsStore();

const notify = useNotify();
const { publicLinksharingURL } = useLinksharing();

const model = defineModel<boolean>({ required: true });

const resets: (() => void)[] = [];
function resettableRef<T>(value: T): Ref<T> {
    const thisRef = ref<T>(value) as Ref<T>;
    resets.push(() => thisRef.value = value);
    return thisRef;
}

const step = resettableRef<NewDomainFlowStep>(NewDomainFlowStep.CustomDomain);
const domain = resettableRef<string>('');
const bucket = resettableRef<string | undefined>(undefined);
const accessKeyID = resettableRef<string>('');
const passphrase = resettableRef<string>(bucketsStore.state.passphrase);
const passphraseOption = resettableRef<PassphraseOption>(PassphraseOption.EnterNewPassphrase);
const accessName = resettableRef<string>('');

const innerContent = ref<Component>();
const isFetching = ref<boolean>(true);
const isGenerating = ref<boolean>(false);

/**
 * Indicates whether the user should be prompted to enter the project passphrase.
 */
const isPromptForPassphrase = computed<boolean>(() => bucketsStore.state.promptForPassphrase);

const hasManagedPassphrase = computed<boolean>(() => projectsStore.state.selectedProjectConfig.hasManagedPassphrase);

const stepInfos: Record<NewDomainFlowStep, StepInfo> = {
    [NewDomainFlowStep.CustomDomain]: new StepInfo(
        'Next',
        undefined,
        () => hasManagedPassphrase.value || !isPromptForPassphrase.value
            ? NewDomainFlowStep.SetupCNAME
            : NewDomainFlowStep.SetupDomainAccess,
        async () => {
            if (hasManagedPassphrase.value || !isPromptForPassphrase.value) {
                await generate();
            }
        },
    ),
    [NewDomainFlowStep.SetupDomainAccess]: new StepInfo(
        'Next',
        NewDomainFlowStep.CustomDomain,
        () => {
            if (passphraseOption.value === PassphraseOption.EnterNewPassphrase) return NewDomainFlowStep.EnterNewPassphrase;
            if (passphraseOption.value === PassphraseOption.GenerateNewPassphrase) return NewDomainFlowStep.PassphraseGenerated;

            return NewDomainFlowStep.SetupCNAME;
        },
        async () => {
            if (
                passphraseOption.value === PassphraseOption.EnterNewPassphrase ||
                passphraseOption.value === PassphraseOption.GenerateNewPassphrase
            ) return;

            await generate();
        },
    ),
    [NewDomainFlowStep.EnterNewPassphrase]: new StepInfo(
        'Next',
        NewDomainFlowStep.SetupDomainAccess,
        NewDomainFlowStep.SetupCNAME,
        generate,
    ),
    [NewDomainFlowStep.PassphraseGenerated]: new StepInfo(
        'Next',
        NewDomainFlowStep.SetupDomainAccess,
        NewDomainFlowStep.SetupCNAME,
        generate,
    ),
    [NewDomainFlowStep.SetupCNAME]: new StepInfo(
        'Next',
        undefined,
        NewDomainFlowStep.SetupTXT,
    ),
    [NewDomainFlowStep.SetupTXT]: new StepInfo(
        'Next',
        NewDomainFlowStep.SetupCNAME,
        NewDomainFlowStep.VerifyDomain,
    ),
    [NewDomainFlowStep.VerifyDomain]: new StepInfo(
        'Next',
        NewDomainFlowStep.SetupTXT,
        NewDomainFlowStep.DomainConnected,
    ),
    [NewDomainFlowStep.DomainConnected]: new StepInfo(
        'Finish',
        NewDomainFlowStep.VerifyDomain,
    ),
};

const cname = computed<string>(() => `${publicLinksharingURL.value.split('//').pop() ?? ''}.`);
const storjRoot = computed<string>(() => `storj-root:${bucket.value}`);
const storjAccess = computed<string>(() => `storj-access:${accessKeyID.value}`);
const storjTLS = 'storj-tls:true';
const txt = computed<string[]>(() => [storjRoot.value, storjAccess.value, storjTLS]);

const currentTitle = computed<string>(() => {
    switch (step.value) {
    case NewDomainFlowStep.CustomDomain: return 'Setup Custom Domain';
    case NewDomainFlowStep.SetupDomainAccess || NewDomainFlowStep.EnterNewPassphrase || NewDomainFlowStep.PassphraseGenerated:
        return 'Setup Domain Access';
    case NewDomainFlowStep.SetupCNAME: return 'Setup CNAME';
    case NewDomainFlowStep.SetupTXT: return 'Setup TXT';
    case NewDomainFlowStep.VerifyDomain: return 'Verify Domain';
    case NewDomainFlowStep.DomainConnected: return 'Domain Connected';
    default: return 'Setup Custom Domain';
    }
});

async function generate(): Promise<void> {
    if (!passphrase.value || !bucket.value) {
        throw new Error('Passphrase and bucket must be set before generating access');
    }

    // Re-generate a unique access name each time to avoid conflicts.
    accessName.value = `custom-domain-access-${new Date().toISOString()}`;

    accessKeyID.value = await domainsStore.generateDomainCredentials(accessName.value, bucket.value, passphrase.value);
    await domainsStore.storeDomain({ subdomain: domain.value, prefix: bucket.value, accessID: accessKeyID.value });
    domainsStore.fetchDomains(1, domainsStore.state.cursor.limit).catch(error => {
        notify.notifyError(error, AnalyticsErrorEventSource.NEW_DOMAIN_MODAL);
    });
}

/**
 * Navigates to the next step.
 */
async function nextStep(): Promise<void> {
    const info = stepInfos[step.value];

    if (isGenerating.value || isFetching.value || info.ref.value?.validate?.() === false) return;

    info.ref.value?.onExit?.('next');

    if (info.beforeNext) {
        isGenerating.value = true;

        try {
            await info.beforeNext();
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.NEW_DOMAIN_MODAL);
            return;
        } finally {
            isGenerating.value = false;
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
        if (
            step.value > NewDomainFlowStep.PassphraseGenerated &&
            passphraseOption.value === PassphraseOption.SetMyProjectPassphrase
        ) {
            bucketsStore.setPassphrase(passphrase.value);
            bucketsStore.setPromptForPassphrase(false);
        }

        resets.forEach(reset => reset());
        return;
    }

    isFetching.value = true;

    const projectID = projectsStore.state.selectedProject.id;

    await Promise.all([
        domainsStore.getAllDomainNames(projectID),
        bucketsStore.getAllBucketsNames(projectID),
    ]).catch(error => {
        notify.notifyError(error, AnalyticsErrorEventSource.NEW_DOMAIN_MODAL);
    });

    passphrase.value = bucketsStore.state.passphrase;

    isFetching.value = false;

    stepInfos[step.value].ref.value?.onEnter?.();
});
</script>

<style scoped lang="scss">
.new-domain__window {
    transition: opacity 250ms cubic-bezier(0.4, 0, 0.2, 1);

    &--loading {
        opacity: 0.3;
        transition: opacity 0s;
        pointer-events: none;
    }
}
</style>

// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row>
        <v-col
            v-for="(step, i) in steps"
            :key="i"
            cols="12"
            :md="steps.length === ONBOARDING_STEPPER_STEPS.length ? 6 : 4"
            :lg="steps.length === ONBOARDING_STEPPER_STEPS.length ? 3 : 4"
            :xl="steps.length === ONBOARDING_STEPPER_STEPS.length ? 3 : 4"
        >
            <v-card class="pa-5 pt-3 h-100 d-flex flex-column">
                <div class="flex-grow-1">
                    <p class="text-overline text-medium-emphasis">
                        {{ step?.stepTxt }}
                    </p>
                    <h4>
                        {{ step?.title }}
                    </h4>
                    <p class="mt-1 mb-2">
                        {{ step?.description }}
                    </p>
                </div>
                <div class="flex-shrink-0">
                    <v-btn
                        :color="step?.color"
                        :variant="step?.variant"
                        :disabled="step?.disabled"
                        :prepend-icon="step?.prependIcon"
                        :append-icon="step?.appendIcon"
                        @click="step?.onClick"
                    >
                        {{ step?.buttonTxt }}
                    </v-btn>
                </div>
            </v-card>
        </v-col>
    </v-row>

    <AccessSetupDialog
        v-model="isAccessDialogOpen"
        :default-step="SetupStep.ChooseAccessStep"
        docs-link="https://docs.storj.io/dcs/access"
        @access-created="onAccessCreated"
    />
    <CreateBucketDialog
        v-model="isBucketDialogOpen"
        @created="onBucketCreated"
    />
    <enter-bucket-passphrase-dialog
        v-if="currentStep === OnboardingStep.UploadFiles || currentStep === OnboardingStep.CreateAccess"
        v-model="isBucketPassphraseDialogOpen"
        @passphrase-entered="passphraseDialogCallback"
    />
    <manage-passphrase-dialog
        v-if="currentStep === OnboardingStep.EncryptionPassphrase"
        v-model="isManagePassphraseDialogOpen"
        is-create
        @passphrase-created="progressStep"
    />
</template>

<script setup lang="ts">
import { VBtn, VCard, VCol, VRow } from 'vuetify/components';
import { computed, FunctionalComponent, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { ArrowRight, Check } from 'lucide-vue-next';

import { useProjectsStore } from '@/store/modules/projectsStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { ONBOARDING_STEPPER_STEPS, OnboardingStep } from '@/types/users';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { EdgeCredentials } from '@/types/accessGrants';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useNotify } from '@/composables/useNotify';
import { ROUTES } from '@/router';
import { OnboardingInfo } from '@/types/common';
import { SetupStep } from '@/types/setupAccess';
import { usePreCheck } from '@/composables/usePreCheck';
import { useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useConfigStore } from '@/store/modules/configStore';

import CreateBucketDialog from '@/components/dialogs/CreateBucketDialog.vue';
import EnterBucketPassphraseDialog from '@/components/dialogs/EnterBucketPassphraseDialog.vue';
import ManagePassphraseDialog from '@/components/dialogs/ManagePassphraseDialog.vue';
import AccessSetupDialog from '@/components/dialogs/AccessSetupDialog.vue';

interface StepData {
    stepTxt: string;
    title: string;
    description: string;
    buttonTxt: string;
    color: string;
    variant: VBtn['$props']['variant'];
    disabled: boolean;
    prependIcon?: FunctionalComponent;
    appendIcon?: FunctionalComponent;
    onClick: () => void;
}

const analyticsStore = useAnalyticsStore();
const bucketsStore = useBucketsStore();
const configStore = useConfigStore();
const obStore = useObjectBrowserStore();
const projectsStore = useProjectsStore();
const userStore = useUsersStore();

const notify = useNotify();
const router = useRouter();
const { withTrialCheck, withManagedPassphraseCheck } = usePreCheck();

let passphraseDialogCallback: () => void = () => {};

const trackedBucketName = ref('');
const isBucketDialogOpen = ref(false);
const isAccessDialogOpen = ref(false);
const isBucketPassphraseDialogOpen = ref(false);
const isManagePassphraseDialogOpen = ref(false);

/**
 * Returns whether this project has passphrase managed by the satellite.
 */
const hasManagedPassphrase = computed((): boolean => {
    return projectsStore.state.selectedProjectConfig.hasManagedPassphrase;
});

const steps = computed<StepData[]>(() => {
    let onBoardSteps = ONBOARDING_STEPPER_STEPS;
    if (hasManagedPassphrase.value) {
        onBoardSteps = onBoardSteps.filter(s => s !== OnboardingStep.EncryptionPassphrase);
    }
    return onBoardSteps.map<StepData>((step, i) => {
        const data: StepData = {
            stepTxt: `Step ${i + 1}`,
            color: currentStep.value === step ? 'primary' : 'default',
            variant: currentStep.value === step ? 'elevated' : 'tonal',
            disabled: currentStep.value !== step,
            buttonTxt: '', description: '', onClick(): void {}, title: '',
        };
        switch (step) {
        case OnboardingStep.EncryptionPassphrase:
            return {
                ...data,
                title: 'Encryption Passphrase',
                description: 'This passphrase will be used to encrypt all your data in this project.',
                buttonTxt: 'Set a Passphrase',
                prependIcon: isPassphraseDone.value ? Check : undefined,
                onClick: onManagePassphrase,
            };
        case OnboardingStep.CreateBucket:
            return {
                ...data,
                title : 'Create a Bucket',
                description : 'Buckets are used to store and organize your data in this project.',
                buttonTxt : 'Create a Bucket',
                prependIcon : isBucketDone.value ? Check : undefined,
                onClick : onCreateBucket,
            };
        case OnboardingStep.UploadFiles:
            return {
                ...data,
                title: 'Upload Files',
                description: 'You are ready to upload files in the bucket you created.',
                buttonTxt: 'Go to Upload',
                color: uploadStepInfo.value.color,
                variant: uploadStepInfo.value.variant as VBtn['$props']['variant'],
                disabled: uploadStepInfo.value.disabled,
                appendIcon: uploadStepInfo.value.appendIcon,
                onClick: uploadFilesClicked,
            };
        case OnboardingStep.CreateAccess:
            return {
                ...data,
                stepTxt: `Step ${i + 1} ${!onboardingInfo.value ? '(Optional)' : ''}`,
                title: onboardingInfo.value?.accessTitle || 'Connect Applications',
                description: onboardingInfo.value?.accessText || `Connect your S3 compatible application to ${configStore.brandName} with S3 credentials.`,
                buttonTxt: onboardingInfo.value?.accessBtnText || 'View Applications',
                color: accessStepInfo.value.color,
                variant: accessStepInfo.value.variant as VBtn['$props']['variant'],
                disabled: accessStepInfo.value.disabled,
                appendIcon: accessStepInfo.value.appendIcon,
                onClick: openAccessDialog,
            };
        default:
            return data;
        }
    });
});

/**
 * contains custom texts to be shown on the steps
 * based on a configured partner. This will remain
 * undefined if the user is not associated with a partner.
 */
const onboardingInfo = computed<OnboardingInfo | null>(() =>
    (configStore.onboardingConfig.get(userStore.state.user.partner ?? '') ?? null) as OnboardingInfo | null,
);

const userSettings = computed(() => userStore.state.settings);

const currentStep = computed<OnboardingStep>(() => {
    return ONBOARDING_STEPPER_STEPS.find(s => s === userSettings.value.onboardingStep) ?? ONBOARDING_STEPPER_STEPS[0];
});

const currentStepIndex = computed(() => ONBOARDING_STEPPER_STEPS.findIndex(s => s === currentStep.value));

/**
 * Returns condition if passphrase step is done.
 */
const isPassphraseDone = computed(() => {
    const passphraseIndex = ONBOARDING_STEPPER_STEPS.findIndex(s => s === OnboardingStep.EncryptionPassphrase);
    return currentStepIndex.value > passphraseIndex;
});

/**
 * Returns condition if create bucket step is done.
 */
const isBucketDone = computed(() => {
    const bucketStepIndex = ONBOARDING_STEPPER_STEPS.findIndex(s => s === OnboardingStep.CreateBucket);
    return currentStepIndex.value > bucketStepIndex;
});

const accessStepInfo = computed(() => {
    const isRelevantStep = currentStep.value === OnboardingStep.CreateAccess
        || currentStep.value === OnboardingStep.UploadFiles;
    const color = 'default';
    const variant = isRelevantStep ? (onboardingInfo.value ? 'elevated' : 'outlined') : 'tonal';
    const disabled = !isRelevantStep;
    const appendIcon = onboardingInfo.value ? undefined : ArrowRight;
    return {
        color,
        variant,
        disabled,
        appendIcon,
    };
});

const uploadStepInfo = computed(() => {
    const isRelevantStep = currentStep.value === OnboardingStep.CreateAccess
        || currentStep.value === OnboardingStep.UploadFiles;
    const color = isRelevantStep ? 'primary' : 'default';
    const variant = isRelevantStep ? (onboardingInfo.value ? 'outlined' : 'elevated') : 'tonal';
    const disabled = !isRelevantStep;
    return {
        color,
        variant,
        disabled,
        appendIcon: ArrowRight,
    };
});

const selectedProject = computed(() => projectsStore.state.selectedProject);

/**
 * Returns condition if user has to be prompt for passphrase from store.
 */
const promptForPassphrase = computed((): boolean => {
    return bucketsStore.state.promptForPassphrase;
});

/**
 * Returns edge credentials from store.
 */
const edgeCredentials = computed((): EdgeCredentials => {
    return bucketsStore.state.edgeCredentials;
});

/**
 * Starts set passphrase flow if user's free trial is not expired.
 */
function onManagePassphrase(): void {
    withTrialCheck(() => { withManagedPassphraseCheck(() => {
        isManagePassphraseDialogOpen.value = true;
    });});
}

/**
 * Starts create bucket flow if user's free trial is not expired.
 */
function onCreateBucket(): void {
    withTrialCheck(() => {
        isBucketDialogOpen.value = true;
    });
}

/**
 * Opens the object browser for the bucket being tracked in onboarding if any
 * or select the only bucket the user has created
 * or redirect to the buckets list.
 */
function uploadFilesClicked(): void {
    withTrialCheck(async () => { withManagedPassphraseCheck(async () => {
        if (trackedBucketName.value) {
            await openTrackedBucket();
        } else {
            await bucketsStore.getBuckets(1, projectsStore.state.selectedProject.id);
            const buckets = bucketsStore.state.page.buckets;
            if (buckets.length === 1) {
                trackedBucketName.value = buckets[0].name;
                await openTrackedBucket();
            } else {
                await progressStep();
                await router.push({
                    name: ROUTES.Buckets.name,
                    params: { id: selectedProject.value.urlId },
                });
            }
        }
    });});
}

/**
 * Opens the object browser for the bucket being tracked in onboarding.
 * This dialog could've been created from the bucket creation step,
 * or selected from the buckets list.
 */
async function openTrackedBucket(): Promise<void> {
    bucketsStore.setFileComponentBucketName(trackedBucketName.value);
    if (!promptForPassphrase.value) {
        if (!edgeCredentials.value.accessKeyId) {
            try {
                await bucketsStore.setS3Client(selectedProject.value.id);
            } catch (error) {
                notify.notifyError(error, AnalyticsErrorEventSource.ONBOARDING_STEPPER);
                return;
            }
        }

        const objCount = bucketsStore.state.page.buckets?.find((bucket) => bucket.name === trackedBucketName.value)?.objectCount ?? 0;
        obStore.setObjectCountOfSelectedBucket(objCount);

        await router.push({
            name: ROUTES.Bucket.name,
            params: {
                browserPath: bucketsStore.state.fileComponentBucketName,
                id: selectedProject.value.urlId,
            },
        });
        await progressStep();
        return;
    }
    passphraseDialogCallback = () => openTrackedBucket();
    isBucketPassphraseDialogOpen.value = true;
}

function onBucketCreated(bucketName: string): void {
    trackedBucketName.value = bucketName;
    progressStep();
}

function openAccessDialog(): void {
    withTrialCheck(async () => { withManagedPassphraseCheck(async () => {
        if (!onboardingInfo.value) {
            router.push({ name: ROUTES.Applications.name, params: { id: selectedProject.value.urlId } });
            return;
        }
        if (currentStep.value === OnboardingStep.UploadFiles) {
            // progress to access step so the onboarding will
            // end correctly after the access is created.
            await progressStep();
        }
        isAccessDialogOpen.value = true;
    });});
}

function onAccessCreated(): void {
    // arbitrary delay so the disappear animation
    // of the access dialog is visible
    setTimeout(() => progressStep(true), 500);
}

/**
 * Progresses from one onboarding stepper step to another
 * and saves the progress in the user settings, conditionally
 * ending the onboarding.
 */
async function progressStep(onboardingEnd = false): Promise<void> {
    let onboardingStep = currentStep.value;
    switch (userSettings.value.onboardingStep) {
    case OnboardingStep.EncryptionPassphrase:
        onboardingStep = OnboardingStep.CreateBucket;
        break;
    case OnboardingStep.CreateBucket:
        onboardingStep = OnboardingStep.UploadFiles;
        break;
    case OnboardingStep.UploadFiles:
        onboardingStep = OnboardingStep.CreateAccess;
        break;
    case OnboardingStep.CreateAccess:
        // no next step from this, but onboardingEnd might be true.
        break;
    default:
        return;
    }
    try {
        await userStore.updateSettings({ onboardingStep, onboardingEnd });
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.ONBOARDING_STEPPER);
    }

    if (onboardingEnd) {
        analyticsStore.eventTriggered(AnalyticsEvent.ONBOARDING_COMPLETED);
    }
}

/**
 * Dismisses the onboarding stepper and abandons the onboarding process.
 */
async function endOnboarding(): Promise<void> {
    try {
        await userStore.updateSettings({ onboardingEnd: true });
        analyticsStore.eventTriggered(AnalyticsEvent.ONBOARDING_ABANDONED);
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.ONBOARDING_STEPPER);
    }
}

watch(() => projectsStore.state.selectedProjectConfig, config => {
    const hasSatelliteManagedEncryption = config.hasManagedPassphrase;
    if (hasSatelliteManagedEncryption && currentStep.value === OnboardingStep.EncryptionPassphrase) {
    // Skip the passphrase step if the project passphrase is satellite managed
        progressStep();
    }
}, { immediate: true });

watch(() => userStore.state.user.partner, async (newPartner) => {
    if (!newPartner) return;

    try {
        await configStore.getPartnerOnboardingConfig(newPartner);
    } catch { /* empty */ }
}, { immediate: true });

defineExpose({ endOnboarding });
</script>

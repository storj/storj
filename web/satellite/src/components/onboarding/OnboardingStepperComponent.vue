// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row>
        <v-col cols="12" md="6" lg="3" xl="3">
            <v-card class="pa-5 pt-3">
                <p class="text-overline">
                    Step 1
                </p>
                <h4>
                    Encryption Passphrase
                </h4>
                <p class="mt-1 mb-2">
                    This passphrase will be used to
                    encrypt all your data in this project.
                </p>
                <v-btn
                    :color="currentStep === OnboardingStep.EncryptionPassphrase ? 'primary' : 'default'"
                    :variant="currentStep === OnboardingStep.EncryptionPassphrase ? 'elevated' : 'tonal'"
                    :disabled="currentStep !== OnboardingStep.EncryptionPassphrase"
                    :prepend-icon="isPassphraseDone ? mdiCheck : ''"
                    block
                    @click="isManagePassphraseDialogOpen = true"
                >
                    Set a Passphrase
                </v-btn>
            </v-card>
        </v-col>
        <v-col cols="12" md="6" lg="3" xl="3">
            <v-card class="pa-5 pt-3">
                <p class="text-overline">
                    Step 2
                </p>
                <h4>
                    Create a Bucket
                </h4>
                <p class="mt-1 mb-2">
                    Buckets are used to store and
                    organize your data in this project.
                </p>
                <v-btn
                    :color="currentStep === OnboardingStep.CreateBucket ? 'primary' : 'default'"
                    :variant="currentStep === OnboardingStep.CreateBucket ? 'elevated' : 'tonal'"
                    :disabled="currentStep !== OnboardingStep.CreateBucket"
                    :prepend-icon="isBucketDone ? mdiCheck : ''"
                    block
                    @click="isBucketDialogOpen = true"
                >
                    Create a Bucket
                </v-btn>
            </v-card>
        </v-col>
        <v-col cols="12" md="6" lg="3" xl="3">
            <v-card class="pa-5 pt-3">
                <p class="text-overline">Step 3</p>
                <h4>Upload Files</h4>
                <p class="mt-1 mb-2">
                    You are ready to upload files
                    in your bucket, and share with the world.
                </p>
                <v-btn
                    :color="uploadStepInfo.color"
                    :variant="uploadStepInfo.variant"
                    :disabled="uploadStepInfo.disabled"
                    router-link
                    block
                    @click="uploadFilesClicked"
                >
                    <template #prepend>
                        <IconUpload />
                    </template>

                    Upload Files
                </v-btn>
            </v-card>
        </v-col>
        <v-col cols="12" md="6" lg="3" xl="3">
            <v-card class="pa-5 pt-3">
                <p class="text-overline">{{ onboardingInfo ? "Step 4" : "Optional" }}</p>
                <h4>{{ onboardingInfo?.accessTitle || "S3 Credentials" }}</h4>
                <p class="mt-1 mb-2">
                    {{ onboardingInfo?.accessText || "Connect your S3 compatible application to Storj with S3 credentials." }}
                </p>
                <v-btn
                    :color="accessStepInfo.color"
                    :variant="accessStepInfo.variant"
                    :disabled="accessStepInfo.disabled"
                    :append-icon="accessStepInfo.appendIcon"
                    block
                    router-link
                    @click="openAccessDialog"
                >
                    {{ onboardingInfo?.accessBtnText || "Create Access Key" }}
                </v-btn>
            </v-card>
        </v-col>
    </v-row>

    <new-access-dialog
        v-if="currentStep === OnboardingStep.UploadFiles || currentStep === OnboardingStep.CreateAccess"
        ref="accessDialog"
        v-model="isAccessDialogOpen"
        @access-created="onAccessCreated"
    />
    <CreateBucketDialog
        v-if="currentStep === OnboardingStep.CreateBucket"
        v-model="isBucketDialogOpen"
        :open-created="false"
        @created="onBucketCreated"
    />
    <enter-bucket-passphrase-dialog
        v-if="currentStep === OnboardingStep.UploadFiles || currentStep === OnboardingStep.CreateAccess"
        v-model="isBucketPassphraseDialogOpen"
        @passphraseEntered="passphraseDialogCallback"
    />
    <manage-passphrase-dialog
        v-if="currentStep === OnboardingStep.EncryptionPassphrase"
        v-model="isManagePassphraseDialogOpen"
        :is-create="true"
        @passphrase-created="progressStep"
    />
</template>

<script setup lang="ts">
import { VBtn, VCard, VCol, VRow } from 'vuetify/components';
import { mdiArrowRight, mdiCheck } from '@mdi/js';
import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';

import { useProjectsStore } from '@/store/modules/projectsStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { ONBOARDING_STEPPER_STEPS, OnboardingStep, User } from '@/types/users';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { EdgeCredentials } from '@/types/accessGrants';
import {
    AnalyticsErrorEventSource,
    AnalyticsEvent,
} from '@/utils/constants/analyticsEventNames';
import { RouteConfig } from '@/types/router';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useNotify } from '@/utils/hooks';
import { ROUTES } from '@/router';
import { OnboardingInfo } from '@/types/common';
import { AccessType } from '@/types/createAccessGrant';

import CreateBucketDialog from '@/components/dialogs/CreateBucketDialog.vue';
import NewAccessDialog from '@/components/dialogs/CreateAccessDialog.vue';
import EnterBucketPassphraseDialog
    from '@/components/dialogs/EnterBucketPassphraseDialog.vue';
import IconUpload from '@/components/icons/IconUpload.vue';
import ManagePassphraseDialog from '@/components/dialogs/ManagePassphraseDialog.vue';

const analyticsStore = useAnalyticsStore();
const bucketsStore = useBucketsStore();
const projectsStore = useProjectsStore();
const userStore = useUsersStore();

const notify = useNotify();
const router = useRouter();

let passphraseDialogCallback: () => void = () => {};

const trackedBucketName = ref('');
const isBucketDialogOpen = ref(false);
const isAccessDialogOpen = ref(false);
const isBucketPassphraseDialogOpen = ref(false);
const isManagePassphraseDialogOpen = ref(false);

const accessDialog = ref<{ setTypes: (newTypes: AccessType[]) => void }>();

/**
 * contains custom texts to be shown on the steps
 * based on a configured partner. This will remain
 * undefined if the user is not associated with a partner.
 */
const onboardingInfo = ref<OnboardingInfo>();

const userSettings = computed(() => userStore.state.settings);

const currentStep = computed<OnboardingStep>(() => {
    if (!ONBOARDING_STEPPER_STEPS.find(s => userStore.state.settings.onboardingStep === s)) {
        return ONBOARDING_STEPPER_STEPS[0];
    }
    return userSettings.value.onboardingStep as OnboardingStep;
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
    const color = isRelevantStep ? 'primary' : 'default';
    const variant = onboardingInfo.value ? 'elevated' : 'outlined';
    const disabled = !isRelevantStep;
    const appendIcon = onboardingInfo.value ? '' : mdiArrowRight;
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
    const variant = onboardingInfo.value ? 'outlined' : 'elevated';
    const disabled = !isRelevantStep;
    return {
        color,
        variant,
        disabled,
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
 * Opens the file browser for the bucket being tracked in onboarding if any
 * or select the only bucket the user has created
 * or redirect to the buckets list.
 */
async function uploadFilesClicked() {
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
}

/**
 * Opens the file browser for the bucket being tracked in onboarding.
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

        analyticsStore.pageVisit(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);
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

function onBucketCreated(bucketName: string) {
    trackedBucketName.value = bucketName;
    progressStep();
}

async function openAccessDialog() {
    if (currentStep.value === OnboardingStep.UploadFiles) {
        // progress to access step so the onboarding will
        // end correctly after the access is created.
        await progressStep();
    }
    accessDialog.value?.setTypes([AccessType.S3]);
    isAccessDialogOpen.value = true;
}

function onAccessCreated() {
    // arbitrary delay so the disappear animation
    // of the access dialog is visible
    setTimeout(() => progressStep(true), 500);
}

/**
 * Progresses from one onboarding stepper step to another
 * and saves the progress in the user settings, conditionally
 * ending the onboarding.
 */
async function progressStep(onboardingEnd = false) {
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

onMounted(async () => {
    const user: User = userStore.state.user;
    if (!user.partner) {
        return;
    }

    try {
        const config = (await import('@/configs/onboardingConfig.json')).default;
        onboardingInfo.value = config[user.partner] as OnboardingInfo;
    } catch { /* empty */ }
});
</script>
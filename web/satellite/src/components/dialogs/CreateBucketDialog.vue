// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        max-width="450px"
        transition="fade-transition"
        persistent
    >
        <v-card ref="innerContent">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <img src="@/assets/icon-bucket.svg" alt="Bucket icon">
                        </v-sheet>
                    </template>

                    <v-card-title class="font-weight-bold">
                        New Bucket
                    </v-card-title>

                    <v-card-subtitle class="text-caption pb-0">
                        Step {{ stepNumber }}: {{ stepName }}
                    </v-card-subtitle>

                    <template #append>
                        <v-btn
                            icon="$close"
                            variant="text"
                            size="small"
                            color="default"
                            :disabled="isLoading"
                            @click="model = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-window v-model="step">
                <v-window-item :value="CreateStep.Name">
                    <v-form v-model="formValid" class="pa-6 pb-3" @submit.prevent>
                        <v-row>
                            <v-col>
                                <p>Buckets are used to store and organize your objects. Enter a bucket name using lowercase characters.</p>
                                <v-text-field
                                    id="Bucket Name"
                                    v-model="bucketName"
                                    variant="outlined"
                                    :rules="bucketNameRules"
                                    label="Bucket name"
                                    placeholder="my-bucket"
                                    hint="Allowed characters [a-z] [0-9] [-.]"
                                    :hide-details="false"
                                    required
                                    autofocus
                                    class="mt-7 mb-3"
                                    minlength="3"
                                    maxlength="63"
                                />
                            </v-col>
                        </v-row>
                    </v-form>
                </v-window-item>

                <v-window-item :value="CreateStep.ObjectLock">
                    <v-form v-model="formValid" class="pa-7" @submit.prevent>
                        <v-row>
                            <v-col>
                                <p class="font-weight-bold mb-2">Do you need object lock?</p>
                                <p>Enabling object lock will prevent objects from being deleted or overwritten for a specified period of time.</p>
                                <v-chip-group
                                    v-model="enableObjectLock"
                                    filter
                                    selected-class="font-weight-bold"
                                    class="mt-2 mb-2"
                                    mandatory
                                >
                                    <v-chip
                                        variant="outlined"
                                        filter
                                        color="default"
                                        :value="false"
                                    >
                                        No
                                    </v-chip>
                                    <v-chip
                                        variant="outlined"
                                        filter
                                        color="default"
                                        :value="true"
                                    >
                                        Yes
                                    </v-chip>
                                </v-chip-group>
                                <v-alert v-if="enableObjectLock" variant="tonal" color="default">
                                    <p class="font-weight-bold text-body-2 mb-1">Enable Object Lock (Compliance Mode)</p>
                                    <p class="text-subtitle-2">No user, including the project owner can overwrite, delete, or alter object lock settings.</p>
                                </v-alert>
                                <v-alert v-else variant="tonal" color="default">
                                    <p class="font-weight-bold text-body-2 mb-1">Object Lock Disabled (Default)</p>
                                    <p class="text-subtitle-2">Objects can be deleted or overwritten.</p>
                                </v-alert>
                            </v-col>
                        </v-row>
                    </v-form>
                </v-window-item>

                <v-window-item :value="CreateStep.Versioning">
                    <v-form v-model="formValid" class="pa-6" @submit.prevent>
                        <v-row>
                            <v-col>
                                <p class="font-weight-bold mb-2">Do you want to enable versioning?</p>
                                <p>Enabling object versioning allows you to preserve, retrieve, and restore previous versions of an object, offering protection against unintentional modifications or deletions.</p>
                                <v-chip-group
                                    v-model="enableVersioning"
                                    :disabled="enableObjectLock"
                                    filter
                                    selected-class="font-weight-bold"
                                    class="mt-2 mb-2"
                                    mandatory
                                >
                                    <v-chip
                                        v-if="!enableObjectLock"
                                        variant="outlined"
                                        filter
                                        :value="false"
                                    >
                                        Disabled
                                    </v-chip>
                                    <v-chip
                                        variant="outlined"
                                        filter
                                        :value="true"
                                        color="primary"
                                    >
                                        Enabled
                                    </v-chip>
                                </v-chip-group>
                                <v-alert v-if="enableObjectLock" variant="tonal" color="default" class="mb-3">
                                    <p class="text-subtitle-2 font-weight-bold">Versioning must be enabled for object lock to work.</p>
                                </v-alert>
                                <v-alert v-if="enableVersioning" variant="tonal" color="default">
                                    <p class="text-subtitle-2">Keep multiple versions of each object in the same bucket. Additional storage costs apply for each version.</p>
                                </v-alert>
                                <v-alert v-else variant="tonal" color="default">
                                    <p class="text-subtitle-2">Uploading an object with the same name will overwrite the existing object in this bucket.</p>
                                </v-alert>
                            </v-col>
                        </v-row>
                    </v-form>
                </v-window-item>

                <v-window-item :value="CreateStep.Confirmation">
                    <v-row class="pa-7">
                        <v-col>
                            <p class="mb-4">You are about to create a new bucket with the following settings:</p>
                            <p>Name:</p>
                            <v-chip
                                variant="tonal"
                                value="Disabled"
                                color="default"
                                class="mt-1 mb-4 font-weight-bold"
                            >
                                {{ bucketName }}
                            </v-chip>

                            <template v-if="objectLockUIEnabled">
                                <p>Object Lock:</p>
                                <v-chip
                                    variant="tonal"
                                    value="Disabled"
                                    color="default"
                                    class="mt-1 mb-4 font-weight-bold"
                                >
                                    {{ enableObjectLock ? 'Enabled' : 'Disabled' }}
                                </v-chip>
                            </template>

                            <template v-if="versioningUIEnabled">
                                <p>Versioning:</p>
                                <v-chip
                                    variant="tonal"
                                    value="Disabled"
                                    color="default"
                                    class="mt-1 font-weight-bold"
                                >
                                    {{ enableVersioning ? 'Enabled' : 'Disabled' }}
                                </v-chip>
                            </template>
                        </v-col>
                    </v-row>
                </v-window-item>

                <v-window-item :value="CreateStep.Success">
                    <div class="pa-7">
                        <v-row>
                            <v-col>
                                <p><strong><v-icon :icon="Check" size="small" /> Bucket successfully created.</strong></p>
                                <v-chip
                                    variant="tonal"
                                    value="Disabled"
                                    color="primary"
                                    class="my-4 font-weight-bold"
                                >
                                    {{ bucketName }}
                                </v-chip>
                                <p>You open the bucket and start uploading objects, or close this dialog and get back to view all buckets.</p>
                            </v-col>
                        </v-row>
                    </div>
                </v-window-item>
            </v-window>
            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn :disabled="isLoading" variant="outlined" color="default" block @click="toPrevStep">
                            {{ stepInfos[step].prevText }}
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            :disabled="!formValid"
                            :loading="isLoading"
                            :append-icon="ArrowRight"
                            color="primary"
                            variant="flat"
                            block
                            @click="toNextStep"
                        >
                            {{ stepInfos[step].nextText }}
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { Component, computed, ref, watch, watchEffect } from 'vue';
import {
    VAlert,
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VCardSubtitle,
    VChip,
    VChipGroup,
    VCol,
    VDialog,
    VDivider,
    VForm,
    VRow,
    VSheet,
    VTextField,
    VWindow,
    VWindowItem,
    VIcon,
} from 'vuetify/components';
import { ArrowRight, Check } from 'lucide-vue-next';
import { useRouter } from 'vue-router';

import { useLoading } from '@/composables/useLoading';
import { useConfigStore } from '@/store/modules/configStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { LocalData } from '@/utils/localData';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { StepInfo, ValidationRule } from '@/types/common';
import { Versioning } from '@/types/versioning';
import { ROUTES } from '@/router';

enum CreateStep {
    Name = 1,
    ObjectLock,
    Versioning,
    Confirmation,
    Success,
}

const { isLoading, withLoading } = useLoading();
const notify = useNotify();
const router = useRouter();
const agStore = useAccessGrantsStore();
const projectsStore = useProjectsStore();
const bucketsStore = useBucketsStore();
const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();

const stepInfos = {
    [CreateStep.Name]: new StepInfo<CreateStep>({
        prev: undefined,
        next: () => {
            if (objectLockUIEnabled.value) return CreateStep.ObjectLock;
            if (allowVersioningStep.value) return CreateStep.Versioning;
            return CreateStep.Success;
        },
        beforeNext: async () => {
            if (objectLockUIEnabled.value || allowVersioningStep.value) return;
            await onCreate();
        },
        validate: (): boolean => {
            return formValid.value;
        },
        noRef: true,
    }),
    [CreateStep.ObjectLock]: new StepInfo<CreateStep>({
        prev: () => CreateStep.Name,
        next: () => {
            if (allowVersioningStep.value) return CreateStep.Versioning;
            return CreateStep.Confirmation;
        },
        noRef: true,
    }),
    [CreateStep.Versioning]: new StepInfo<CreateStep>({
        prev: () => {
            if (objectLockUIEnabled.value) return CreateStep.ObjectLock;
            return CreateStep.Name;
        },

        next: () => CreateStep.Confirmation,
        noRef: true,
    }),
    [CreateStep.Confirmation]: new StepInfo<CreateStep>({
        prev: () => {
            if (allowVersioningStep.value) return CreateStep.Versioning;
            if (objectLockUIEnabled.value) return CreateStep.ObjectLock;
            return CreateStep.Name;
        },
        beforeNext: onCreate,
        next: () => CreateStep.Success,
        nextText: 'Create Bucket',
        noRef: true,
    }),
    [CreateStep.Success]: new StepInfo<CreateStep>({
        prevText: 'Close',
        nextText: 'Open Bucket',
        noRef: true,
    }),
};

// Copied from here https://github.com/storj/storj/blob/f6646b0e88700b5e7113a76a8d07bf346b59185a/satellite/metainfo/validation.go#L38
const ipRegexp = /^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$/;

const model = defineModel<boolean>({ required: true });

const emit = defineEmits<{
    (event: 'created', value: string): void,
}>();

const step = ref<CreateStep>(CreateStep.Name);
const stepNumber = ref<number>(1);
const innerContent = ref<Component | null>(null);
const formValid = ref<boolean>(false);
const enableVersioning = ref<boolean>(false);
const enableObjectLock = ref<boolean>(false);
const bucketName = ref<string>('');
const worker = ref<Worker | null>(null);

const project = computed(() => projectsStore.state.selectedProject);

/**
 * Whether versioning has been enabled for current project.
 */
const versioningUIEnabled = computed<boolean>(() => {
    return projectsStore.versioningUIEnabled;
});

/**
 * Whether the versioning step should be shown.
 * Projects with versioning enabled as default should not have this step.
 */
const allowVersioningStep = computed<boolean>(() => {
    return versioningUIEnabled.value  && project.value.versioning !== Versioning.Enabled;
});

/**
 * Whether object lock is enabled for current project.
 */
const objectLockUIEnabled = computed<boolean>(() => {
    return projectsStore.objectLockUIEnabledForProject && configStore.objectLockUIEnabled;
});

const bucketNameRules = computed((): ValidationRule<string>[] => {
    return [
        (value: string) => (!!value || 'Bucket name is required.'),
        (value: string) => ((value.length >= 3 && value.length <= 63) || 'Name should be between 3 and 63 characters length.'),
        (value: string) => {
            const labels = value.split('.');
            for (let i = 0; i < labels.length; i++) {
                const l = labels[i];
                if (!l.length) return 'Bucket name cannot start or end with a dot.';
                if (!/^[a-z0-9]$/.test(l[0])) return 'Bucket name must start with a lowercase letter or number.';
                if (!/^[a-z0-9]$/.test(l[l.length - 1])) return 'Bucket name must end with a lowercase letter or number.';
                if (!/^[a-z0-9-.]+$/.test(l)) return 'Bucket name can contain only lowercase letters, numbers or hyphens.';
            }
            return true;
        },
        (value: string) => (!ipRegexp.test(value) || 'Bucket name cannot be formatted as an IP address.'),
        (value: string) => (!allBucketNames.value.includes(value) || 'A bucket exists with this name.'),
    ];
});

const stepName = computed<string>(() => {
    switch (step.value) {
    case CreateStep.Name:
        return 'Name';
    case CreateStep.ObjectLock:
        return 'Object Lock';
    case CreateStep.Versioning:
        return 'Object Versioning';
    case CreateStep.Confirmation:
        return 'Confirmation';
    case CreateStep.Success:
        return 'Bucket Created';
    default:
        return '';
    }
});

/**
 * Returns all bucket names from store.
 */
const allBucketNames = computed((): string[] => {
    return bucketsStore.state.allBucketNames;
});

/**
 * Returns condition if user has to be prompt for passphrase from store.
 */
const promptForPassphrase = computed((): boolean => {
    return bucketsStore.state.promptForPassphrase;
});

/**
 * Returns object browser api key from store.
 */
const apiKey = computed((): string => {
    return bucketsStore.state.apiKey;
});

/**
 * Returns edge credentials from store.
 */
const edgeCredentials = computed((): EdgeCredentials => {
    return bucketsStore.state.edgeCredentials;
});

/**
 * Returns edge credentials for bucket creation from store.
 */
const edgeCredentialsForCreate = computed((): EdgeCredentials => {
    return bucketsStore.state.edgeCredentialsForCreate;
});

/**
 * Indicates if bucket was created.
 */
const bucketWasCreated = computed((): boolean => {
    const status = LocalData.getBucketWasCreatedStatus();
    if (status !== null) {
        return status;
    }

    return false;
});

/**
 * Sets local worker with worker instantiated in store.
 */
function setWorker(): void {
    worker.value = agStore.state.accessGrantsWebWorker;
    if (worker.value) {
        worker.value.onerror = (error: ErrorEvent) => {
            notify.error(error.message, AnalyticsErrorEventSource.CREATE_BUCKET_MODAL);
        };
    }
}

/**
 * Conditionally close dialog or go to previous step.
 */
function toPrevStep(): void {
    if (step.value === CreateStep.Success) {
        model.value = false;
        return;
    }
    const info = stepInfos[step.value];
    if (info.prev?.value) {
        step.value = info.prev.value;
        stepNumber.value--;
    } else {
        model.value = false;
    }
}

/**
 * Conditionally create bucket or go to next step.
 */
function toNextStep(): void {
    if (!formValid.value) return;

    if (step.value === CreateStep.Success) {
        openBucket();
        return;
    }
    const info = stepInfos[step.value];
    if (info.ref.value?.validate?.() === false) {
        return;
    }
    withLoading(async () => {
        try {
            await info.beforeNext?.();
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.CREATE_BUCKET_MODAL);
            return;
        }
        if (info.next?.value) {
            step.value = info.next.value;
            stepNumber.value++;
        }
    });
}

/**
 * Navigates to bucket page.
 */
async function openBucket(): Promise<void> {
    bucketsStore.setFileComponentBucketName(bucketName.value);
    await router.push({
        name: ROUTES.Bucket.name,
        params: {
            browserPath: bucketsStore.state.fileComponentBucketName,
            id: projectsStore.state.selectedProject.urlId,
        },
    });
}

/**
 * Validates provided bucket's name and creates a bucket.
 */
async function onCreate(): Promise<void> {
    if (!worker.value) {
        notify.error('Worker is not defined', AnalyticsErrorEventSource.CREATE_BUCKET_MODAL);
        return;
    }

    const projectID = project.value.id;

    if (!promptForPassphrase.value) {
        if (!edgeCredentials.value.accessKeyId) {
            await bucketsStore.setS3Client(projectID);
        }
        await bucketsStore.createBucket(bucketName.value, enableObjectLock.value, enableVersioning.value);
        await bucketsStore.getBuckets(1, projectID);
        analyticsStore.eventTriggered(AnalyticsEvent.BUCKET_CREATED);

        if (!bucketWasCreated.value) {
            LocalData.setBucketWasCreatedStatus();
        }

        step.value = CreateStep.Success;
        emit('created', bucketName.value);
        return;
    }

    if (edgeCredentialsForCreate.value.accessKeyId) {
        await bucketsStore.createBucketWithNoPassphrase(bucketName.value, enableObjectLock.value, enableVersioning.value);
        await bucketsStore.getBuckets(1, projectID);
        analyticsStore.eventTriggered(AnalyticsEvent.BUCKET_CREATED);
        if (!bucketWasCreated.value) {
            LocalData.setBucketWasCreatedStatus();
        }

        step.value = CreateStep.Success;
        emit('created', bucketName.value);

        return;
    }

    const now = new Date();

    if (!apiKey.value) {
        const name = `${configStore.state.config.objectBrowserKeyNamePrefix}${now.getTime()}`;
        const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(name, projectID);
        bucketsStore.setApiKey(cleanAPIKey.secret);
    }

    const inOneHour = new Date(now.setHours(now.getHours() + 1));

    worker.value.postMessage({
        'type': 'SetPermission',
        'isDownload': false,
        'isUpload': true,
        'isList': false,
        'isDelete': false,
        'notAfter': inOneHour.toISOString(),
        'buckets': JSON.stringify([]),
        'apiKey': apiKey.value,
    });

    const grantEvent: MessageEvent = await new Promise(resolve => {
        if (worker.value) {
            worker.value.onmessage = resolve;
        }
    });
    if (grantEvent.data.error) {
        notify.error(grantEvent.data.error, AnalyticsErrorEventSource.CREATE_BUCKET_MODAL);
        return;
    }

    const salt = await projectsStore.getProjectSalt(projectID);
    const satelliteNodeURL: string = configStore.state.config.satelliteNodeURL;

    worker.value.postMessage({
        'type': 'GenerateAccess',
        'apiKey': grantEvent.data.value,
        'passphrase': '',
        'salt': salt,
        'satelliteNodeURL': satelliteNodeURL,
    });

    const accessGrantEvent: MessageEvent = await new Promise(resolve => {
        if (worker.value) {
            worker.value.onmessage = resolve;
        }
    });
    if (accessGrantEvent.data.error) {
        notify.error(accessGrantEvent.data.error, AnalyticsErrorEventSource.CREATE_BUCKET_MODAL);
        return;
    }

    const accessGrant = accessGrantEvent.data.value;

    const creds: EdgeCredentials = await agStore.getEdgeCredentials(accessGrant);
    bucketsStore.setEdgeCredentialsForCreate(creds);
    await bucketsStore.createBucketWithNoPassphrase(bucketName.value, enableObjectLock.value, enableVersioning.value);
    await bucketsStore.getBuckets(1, projectID);
    analyticsStore.eventTriggered(AnalyticsEvent.BUCKET_CREATED);

    if (!bucketWasCreated.value) {
        LocalData.setBucketWasCreatedStatus();
    }

    step.value = CreateStep.Success;
    emit('created', bucketName.value);
}

watchEffect(() => {
    if (enableObjectLock.value) {
        enableVersioning.value = true;
    }
});

watch(innerContent, newContent => {
    if (newContent) {
        setWorker();

        withLoading(async () => {
            try {
                await bucketsStore.getAllBucketsNames(project.value.id);
            } catch (error) {
                notify.notifyError(error, AnalyticsErrorEventSource.CREATE_BUCKET_MODAL);
            }
        });
        return;
    }
    // dialog has been closed
    bucketName.value = '';
    step.value = CreateStep.Name;
    stepNumber.value = 1;
    enableVersioning.value = false;
    enableObjectLock.value = false;
});
</script>

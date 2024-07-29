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
                        <!-- <img src="../assets/icon-bucket-color.svg" alt="Bucket" width="40"> -->
                        New Bucket
                    </v-card-title>

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
                    <v-form v-model="formValid" class="pa-6 pb-3" @submit.prevent="createOrNext">
                        <v-row>
                            <v-col>
                                <p>Buckets are used to store and organize your files. Enter a bucket name using lowercase characters.</p>
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
                <v-window-item :value="CreateStep.Versioning">
                    <v-form v-model="versioningValid" class="pa-6">
                        <v-row>
                            <v-col>
                                <p class="font-weight-bold mb-2">Do you want to enable versioning?</p>
                                <p>Enabling object versioning allows you to preserve, retrieve, and restore previous versions of a file, offering protection against unintentional modifications or deletions.</p>
                                <v-chip-group
                                    v-model="enableVersioning"
                                    filter
                                    selected-class="font-weight-bold"
                                    class="mt-2 mb-2"
                                    mandatory
                                >
                                    <v-chip
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
                                <v-alert v-if="enableVersioning" variant="tonal" color="default">
                                    <p class="text-subtitle-2">Keep multiple versions of each file in the same bucket. Additional storage costs apply for each version.</p>
                                </v-alert>
                                <v-alert v-else variant="tonal" color="default">
                                    <p class="text-subtitle-2">Uploading a file with the same name will overwrite the existing file in this bucket.</p>
                                </v-alert>
                            </v-col>
                        </v-row>
                    </v-form>
                </v-window-item>
                <v-window-item :value="CreateStep.Success">
                    <div class="pa-7">
                        <v-row>
                            <v-col>
                                <p><strong>Bucket successfully created.</strong></p>
                                <v-chip
                                    variant="tonal"
                                    value="Disabled"
                                    color="default"
                                    class="my-4 font-weight-bold"
                                >
                                    {{ bucketName }}
                                </v-chip>
                                <p>You can choose to open the bucket and start uploading files, or close this dialog and get back to the buckets page.</p>
                            </v-col>
                        </v-row>
                    </div>
                </v-window-item>
            </v-window>
            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn :disabled="isLoading" variant="outlined" color="default" block @click="closeOrBack">
                            {{ secondaryBtnText }}
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            :disabled="!formValid"
                            :loading="isLoading"
                            :append-icon="step === CreateStep.Success ? ArrowRight : undefined"
                            color="primary"
                            variant="flat"
                            block
                            @click="createOrNext"
                        >
                            {{ primaryBtnText }}
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { Component, computed, ref, watch } from 'vue';
import {
    VAlert,
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
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
} from 'vuetify/components';
import { ArrowRight } from 'lucide-vue-next';
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
import { ValidationRule } from '@/types/common';
import { Versioning } from '@/types/versioning';
import { ROUTES } from '@/router';

enum CreateStep {
    Name = 1,
    Versioning = 2,
    Success = 3,
}

const { isLoading, withLoading } = useLoading();
const notify = useNotify();
const router = useRouter();
const agStore = useAccessGrantsStore();
const projectsStore = useProjectsStore();
const bucketsStore = useBucketsStore();
const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();

// Copied from here https://github.com/storj/storj/blob/f6646b0e88700b5e7113a76a8d07bf346b59185a/satellite/metainfo/validation.go#L38
const ipRegexp = /^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$/;

const model = defineModel<boolean>({ required: true });

const emit = defineEmits<{
    (event: 'created', value: string): void,
}>();

const step = ref(CreateStep.Name);
const innerContent = ref<Component | null>(null);
const formValid = ref<boolean>(false);
const versioningValid = ref<boolean>(false);
const enableVersioning = ref<boolean>(false);
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

const primaryBtnText = computed(() => {
    switch (true) {
    case step.value === CreateStep.Name && !allowVersioningStep.value:
    case step.value === CreateStep.Versioning:
        return 'Create Bucket';
    case step.value === CreateStep.Success:
        return 'Open Bucket';
    }
    return 'Next';
});

const secondaryBtnText = computed(() => {
    switch (true) {
    case step.value === CreateStep.Name:
        return 'Cancel';
    case step.value === CreateStep.Success:
        return 'Close';
    }
    return 'Back';
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
function closeOrBack() {
    switch (true) {
    case step.value === CreateStep.Versioning:
        step.value = CreateStep.Name;
        break;
    case step.value === CreateStep.Name:
    case step.value === CreateStep.Success:
        model.value = false;
    }
}

/**
 * Conditionally create bucket or go to next step.
 */
function createOrNext() {
    switch (true) {
    case step.value === CreateStep.Name && allowVersioningStep.value:
        step.value = CreateStep.Versioning;
        break;
    case step.value === CreateStep.Name && !allowVersioningStep.value:
    case step.value === CreateStep.Versioning:
        onCreate();
        break;
    case step.value === CreateStep.Success:
        model.value = false;
        openBucket();
        break;
    }
}

/**
 * Navigates to bucket page.
 */
async function openBucket() {
    bucketsStore.setFileComponentBucketName(bucketName.value);
    await router.push({
        name: ROUTES.Bucket.name,
        params: {
            browserPath: bucketsStore.state.fileComponentBucketName,
            id: projectsStore.state.selectedProject.urlId,
        },
    });
    return;
}

/**
 * Validates provided bucket's name and creates a bucket.
 */
function onCreate(): void {
    if (!formValid.value) return;

    withLoading(async () => {
        if (!worker.value) {
            notify.error('Worker is not defined', AnalyticsErrorEventSource.CREATE_BUCKET_MODAL);
            return;
        }

        try {
            const projectID = project.value.id;

            if (!promptForPassphrase.value) {
                if (!edgeCredentials.value.accessKeyId) {
                    await bucketsStore.setS3Client(projectID);
                }
                await bucketsStore.createBucket(bucketName.value, enableVersioning.value);
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
                await bucketsStore.createBucketWithNoPassphrase(bucketName.value, enableVersioning.value);
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
            await bucketsStore.createBucketWithNoPassphrase(bucketName.value, enableVersioning.value);
            await bucketsStore.getBuckets(1, projectID);
            analyticsStore.eventTriggered(AnalyticsEvent.BUCKET_CREATED);

            if (!bucketWasCreated.value) {
                LocalData.setBucketWasCreatedStatus();
            }

            step.value = CreateStep.Success;
            emit('created', bucketName.value);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.CREATE_BUCKET_MODAL);
        }
    });
}

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
    enableVersioning.value = false;
});
</script>

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        :persistent="isLoading"
        width="auto"
        min-width="320px"
        max-width="450px"
        transition="fade-transition"
    >
        <v-card ref="innerContent" rounded="xlg">
            <v-sheet>
                <v-card-item class="pa-5 pl-7">
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

            <v-form v-model="formValid" class="pa-7 pb-3" @submit.prevent="onCreate">
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

            <v-divider />

            <v-card-actions class="pa-7">
                <v-row>
                    <v-col>
                        <v-btn :disabled="isLoading" variant="outlined" color="default" block @click="model = false">Cancel</v-btn>
                    </v-col>
                    <v-col>
                        <v-btn :disabled="!formValid" :loading="isLoading" color="primary" variant="flat" block @click="onCreate">
                            Create Bucket
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { Component, computed, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VCol,
    VDialog,
    VDivider,
    VForm,
    VRow, VSheet,
    VTextField,
} from 'vuetify/components';

import { useLoading } from '@/composables/useLoading';
import { useConfigStore } from '@/store/modules/configStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { FILE_BROWSER_AG_NAME, useBucketsStore } from '@/store/modules/bucketsStore';
import { LocalData } from '@/utils/localData';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { ROUTES } from '@/router';

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

const innerContent = ref<Component | null>(null);
const formValid = ref<boolean>(false);
const bucketName = ref<string>('');
const worker = ref<Worker | null>(null);

const bucketNameRules = computed(() => {
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
        },
        (value: string) => {
            if (ipRegexp.test(value)) return 'Bucket name cannot be formatted as an IP address.';
        },
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
            const projectID = projectsStore.state.selectedProject.id;

            if (!promptForPassphrase.value) {
                if (!edgeCredentials.value.accessKeyId) {
                    await bucketsStore.setS3Client(projectID);
                }
                await bucketsStore.createBucket(bucketName.value);
                await bucketsStore.getBuckets(1, projectID);
                analyticsStore.eventTriggered(AnalyticsEvent.BUCKET_CREATED);

                if (!bucketWasCreated.value) {
                    LocalData.setBucketWasCreatedStatus();
                }

                model.value = false;
                emit('created', bucketName.value);
                return;
            }

            if (edgeCredentialsForCreate.value.accessKeyId) {
                await bucketsStore.createBucketWithNoPassphrase(bucketName.value);
                await bucketsStore.getBuckets(1, projectID);
                analyticsStore.eventTriggered(AnalyticsEvent.BUCKET_CREATED);
                if (!bucketWasCreated.value) {
                    LocalData.setBucketWasCreatedStatus();
                }

                model.value = false;
                emit('created', bucketName.value);

                return ;
            }

            if (!apiKey.value) {
                await agStore.deleteAccessGrantByNameAndProjectID(FILE_BROWSER_AG_NAME, projectID);
                const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(FILE_BROWSER_AG_NAME, projectID);
                bucketsStore.setApiKey(cleanAPIKey.secret);
            }

            const now = new Date();
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
            await bucketsStore.createBucketWithNoPassphrase(bucketName.value);
            await bucketsStore.getBuckets(1, projectID);
            analyticsStore.eventTriggered(AnalyticsEvent.BUCKET_CREATED);

            if (!bucketWasCreated.value) {
                LocalData.setBucketWasCreatedStatus();
            }

            model.value = false;
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
                await bucketsStore.getAllBucketsNames(projectsStore.state.selectedProject.id);
            } catch (error) {
                notify.notifyError(error, AnalyticsErrorEventSource.CREATE_BUCKET_MODAL);
            }
        });
        return;
    }
    // dialog has been closed
    bucketName.value = '';
});
</script>

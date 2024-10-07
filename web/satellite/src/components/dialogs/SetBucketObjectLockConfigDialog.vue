// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        :persistent="isLoading"
        width="auto"
        max-width="420px"
        transition="fade-transition"
    >
        <v-card>
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <component :is="Lock" :size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">Lock</v-card-title>
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

            <v-divider />

            <v-form v-model="formValid" class="pa-6" @submit.prevent="onSetLock">
                <v-row>
                    <v-col>
                        <p class="mb-4">
                            Enabling object lock will prevent objects from being deleted or overwritten for a specified period of time.
                        </p>
                        <set-default-object-lock-config
                            :existing-mode="defaultRetentionMode"
                            :existing-period="defaultRetentionPeriod"
                            :existing-period-unit="defaultRetentionPeriodUnit"
                            @updateDefaultMode="newMode => defaultRetentionMode = newMode"
                            @updatePeriodValue="newPeriod => defaultRetentionPeriod = newPeriod"
                            @updatePeriodUnit="newUnit => defaultRetentionPeriodUnit = newUnit"
                        />
                    </v-col>
                </v-row>
            </v-form>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            :disabled="isLoading"
                            @click="model = false"
                        >
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary"
                            variant="flat"
                            block
                            :loading="isLoading"
                            @click="onSetLock"
                        >
                            Set Lock
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VCol,
    VDialog,
    VDivider, VForm,
    VRow,
    VSheet,
} from 'vuetify/components';
import { Lock } from 'lucide-vue-next';
import type { ObjectLockRule } from '@aws-sdk/client-s3';

import { useLoading } from '@/composables/useLoading';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { DefaultObjectLockPeriodUnit, ObjLockMode } from '@/types/objectLock';
import { Bucket } from '@/types/buckets';
import { ClientType, useBucketsStore } from '@/store/modules/bucketsStore';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useConfigStore } from '@/store/modules/configStore';

import SetDefaultObjectLockConfig from '@/components/dialogs/defaultBucketLockConfig/SetDefaultObjectLockConfig.vue';

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const bucketsStore = useBucketsStore();
const projectsStore = useProjectsStore();
const agStore = useAccessGrantsStore();
const configStore = useConfigStore();

const props = defineProps<{
    bucketName: string
}>();

const model = defineModel<boolean>({ required: true });

const formValid = ref<boolean>(false);
const defaultRetentionMode = ref<ObjLockMode>();
const defaultRetentionPeriod = ref<number>(0);
const defaultRetentionPeriodUnit = ref<DefaultObjectLockPeriodUnit>(DefaultObjectLockPeriodUnit.DAYS);
const worker = ref<Worker | null>(null);

const bucketData = computed<Bucket>(() => {
    return bucketsStore.state.page.buckets.find(bucket => bucket.name === props.bucketName) ?? new Bucket();
});

const projectID = computed<string>(() => projectsStore.state.selectedProject.id);
const promptForPassphrase = computed<boolean>(() => bucketsStore.state.promptForPassphrase);
const edgeCredentials = computed<EdgeCredentials>(() => bucketsStore.state.edgeCredentials);
const edgeCredentialsForObjectLock = computed<EdgeCredentials>(() => bucketsStore.state.edgeCredentialsForObjectLock);
const apiKey = computed<string>(() => bucketsStore.state.apiKey);

function onSetLock(): void {
    withLoading(async () => {
        try {
            let rule: ObjectLockRule | undefined = undefined;
            if (defaultRetentionMode.value) {
                rule = {
                    DefaultRetention: {
                        Mode: defaultRetentionMode.value,
                        Days: defaultRetentionPeriodUnit.value === DefaultObjectLockPeriodUnit.DAYS ? defaultRetentionPeriod.value : undefined,
                        Years: defaultRetentionPeriodUnit.value === DefaultObjectLockPeriodUnit.YEARS ? defaultRetentionPeriod.value : undefined,
                    },
                };
            }

            const bucketsPage = bucketsStore.state.cursor.page;
            const bucketsLimit = bucketsStore.state.cursor.limit;

            if (!promptForPassphrase.value) {
                if (!edgeCredentials.value.accessKeyId) {
                    await bucketsStore.setS3Client(projectID.value);
                }
                await bucketsStore.setObjectLockConfig(props.bucketName, ClientType.REGULAR, rule);
                await bucketsStore.getBuckets(bucketsPage, projectID.value, bucketsLimit);

                notify.success('Bucket Object Lock configuration has been updated.');
                model.value = false;
                return;
            }

            if (edgeCredentialsForObjectLock.value.accessKeyId) {
                await bucketsStore.setObjectLockConfig(props.bucketName, ClientType.FOR_OBJECT_LOCK, rule);
                await bucketsStore.getBuckets(bucketsPage, projectID.value, bucketsLimit);

                notify.success('Bucket Object Lock configuration has been updated.');
                model.value = false;
                return;
            }

            if (!worker.value) {
                notify.error('Worker is not defined', AnalyticsErrorEventSource.SET_BUCKET_OBJECT_LOCK_CONFIG_MODAL);
                return;
            }

            const now = new Date();

            if (!apiKey.value) {
                const name = `${configStore.state.config.objectBrowserKeyNamePrefix}${now.getTime()}`;
                const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(name, projectID.value);
                bucketsStore.setApiKey(cleanAPIKey.secret);
            }

            const inOneHour = new Date(now.setHours(now.getHours() + 1));

            worker.value.postMessage({
                'type': 'SetPermission',
                'isDownload': false,
                'isUpload': true,
                'isList': false,
                'isDelete': false,
                'isPutObjectLockConfiguration': true,
                'isGetObjectLockConfiguration': true,
                'notAfter': inOneHour.toISOString(),
                'buckets': JSON.stringify([]),
                'apiKey': apiKey.value,
            });

            const grantEvent: MessageEvent = await new Promise(resolve => {
                if (worker.value) worker.value.onmessage = resolve;
            });
            if (grantEvent.data.error) {
                notify.error(grantEvent.data.error, AnalyticsErrorEventSource.SET_BUCKET_OBJECT_LOCK_CONFIG_MODAL);
                return;
            }

            const salt = await projectsStore.getProjectSalt(projectID.value);
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
                notify.error(accessGrantEvent.data.error, AnalyticsErrorEventSource.SET_BUCKET_OBJECT_LOCK_CONFIG_MODAL);
                return;
            }

            const accessGrant = accessGrantEvent.data.value;

            const creds: EdgeCredentials = await agStore.getEdgeCredentials(accessGrant);
            bucketsStore.setEdgeCredentialsForObjectLock(creds);
            await bucketsStore.setObjectLockConfig(props.bucketName, ClientType.FOR_OBJECT_LOCK, rule);
            await bucketsStore.getBuckets(bucketsPage, projectID.value, bucketsLimit);

            notify.success('Bucket Object Lock configuration has been updated.');
            model.value = false;
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.SET_BUCKET_OBJECT_LOCK_CONFIG_MODAL);
        }
    });
}

function setWorker(): void {
    worker.value = agStore.state.accessGrantsWebWorker;
    if (worker.value) {
        worker.value.onerror = (error: ErrorEvent) => {
            notify.error(error.message, AnalyticsErrorEventSource.SET_BUCKET_OBJECT_LOCK_CONFIG_MODAL);
        };
    }
}

watch(model, value => {
    if (value) {
        setWorker();

        defaultRetentionMode.value = bucketData.value.defaultRetentionMode === 'Not set' ? undefined : bucketData.value.defaultRetentionMode;

        if (bucketData.value.defaultRetentionYears) {
            defaultRetentionPeriodUnit.value = DefaultObjectLockPeriodUnit.YEARS;
            defaultRetentionPeriod.value = bucketData.value.defaultRetentionYears;
        } else if (bucketData.value.defaultRetentionDays) {
            defaultRetentionPeriodUnit.value = DefaultObjectLockPeriodUnit.DAYS;
            defaultRetentionPeriod.value = bucketData.value.defaultRetentionDays;
        }
    } else {
        defaultRetentionMode.value = undefined;
        defaultRetentionPeriod.value = 0;
        defaultRetentionPeriodUnit.value = DefaultObjectLockPeriodUnit.DAYS;
    }
});
</script>

// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        :persistent="isLoading"
        width="auto"
        max-width="440px"
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
                <v-card-title class="font-weight-bold">Lock Settings</v-card-title>
                <template #append>
                    <v-btn
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-form ref="form" v-model="formValid" class="pa-6" @submit.prevent="onSetLock">
                <v-row>
                    <v-col>
                        <p v-if="bucketData.objectLockEnabled" class="mb-4">
                            Object Lock is enabled on this bucket. This setting cannot be disabled.
                        </p>
                        <p v-else class="mb-4">
                            Object Lock is disabled on this bucket.
                        </p>
                        <set-default-object-lock-config
                            v-model:default-retention-period="defaultRetentionPeriod"
                            v-model:default-retention-mode="defaultRetentionMode"
                            v-model:period-unit="defaultRetentionPeriodUnit"
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
                            :disabled="!formValid"
                            :loading="isLoading"
                            @click="onSetLock"
                        >
                            Save Changes
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
    VDivider,
    VForm,
    VRow,
    VSheet,
} from 'vuetify/components';
import { Lock, X } from 'lucide-vue-next';
import type { ObjectLockRule } from '@aws-sdk/client-s3';

import { useLoading } from '@/composables/useLoading';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { DefaultObjectLockPeriodUnit, NO_MODE_SET, ObjLockMode } from '@/types/objectLock';
import { Bucket } from '@/types/buckets';
import { ClientType, useBucketsStore } from '@/store/modules/bucketsStore';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAccessGrantWorker } from '@/composables/useAccessGrantWorker';

import SetDefaultObjectLockConfig from '@/components/dialogs/defaultBucketLockConfig/SetDefaultObjectLockConfig.vue';

const { isLoading, withLoading } = useLoading();
const notify = useNotify();
const { setPermissions, generateAccess } = useAccessGrantWorker();

const bucketsStore = useBucketsStore();
const projectsStore = useProjectsStore();
const agStore = useAccessGrantsStore();
const configStore = useConfigStore();

const props = defineProps<{
    bucketName: string
}>();

const model = defineModel<boolean>({ required: true });

const form = ref<VForm>();
const formValid = ref<boolean>(false);
const defaultRetentionMode = ref<ObjLockMode | typeof NO_MODE_SET>(NO_MODE_SET);
const defaultRetentionPeriod = ref<number>(0);
const defaultRetentionPeriodUnit = ref<DefaultObjectLockPeriodUnit>(DefaultObjectLockPeriodUnit.DAYS);

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
            if (defaultRetentionMode.value !== NO_MODE_SET) {
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

            const now = new Date();

            if (!apiKey.value) {
                const name = `${configStore.state.config.objectBrowserKeyNamePrefix}${now.getTime()}`;
                const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(name, projectID.value);
                bucketsStore.setApiKey(cleanAPIKey.secret);
            }

            const inOneHour = new Date(now.setHours(now.getHours() + 1));

            const macaroon = await setPermissions({
                isDownload: false,
                isUpload: true,
                isList: false,
                isDelete: false,
                isPutObjectLockConfiguration: true,
                isGetObjectLockConfiguration: true,
                notAfter: inOneHour.toISOString(),
                buckets: JSON.stringify([]),
                apiKey: apiKey.value,
            });

            const accessGrant = await generateAccess({
                apiKey: macaroon,
                passphrase: '',
            }, projectID.value);

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

watch(model, value => {
    if (value) {
        defaultRetentionMode.value = bucketData.value.defaultRetentionMode;

        if (bucketData.value.defaultRetentionYears) {
            defaultRetentionPeriodUnit.value = DefaultObjectLockPeriodUnit.YEARS;
            defaultRetentionPeriod.value = bucketData.value.defaultRetentionYears;
        } else if (bucketData.value.defaultRetentionDays) {
            defaultRetentionPeriodUnit.value = DefaultObjectLockPeriodUnit.DAYS;
            defaultRetentionPeriod.value = bucketData.value.defaultRetentionDays;
        }
    } else {
        defaultRetentionMode.value = NO_MODE_SET;
        defaultRetentionPeriod.value = 0;
        defaultRetentionPeriodUnit.value = DefaultObjectLockPeriodUnit.DAYS;
    }
});

watch([model, form], async () => {
    if (model.value && form.value) {
        const result = await form.value.validate();
        formValid.value = result.valid;
    }
});
</script>

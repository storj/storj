// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="420px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card>
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center text-error"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <component :is="Trash2" :size="18" />
                        </v-sheet>
                    </template>
                    <v-card-title class="font-weight-bold text-error">
                        Delete Bucket
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

            <v-card-item v-if="!hasObjects" class="pa-6">
                <p class="mb-3">
                    <b>
                        The following bucket and all of its data
                        {{ bucket?.versioning !== Versioning.NotSupported ? "(including versions)" : '' }}
                        will be deleted. Data will not be recoverable.
                    </b>
                </p>

                <p class="my-2">
                    <v-chip
                        variant="tonal"
                        filter
                        color="default"
                        class="font-weight-bold text-wrap h-100 py-2 mr-2"
                    >
                        {{ bucketName }}
                    </v-chip>
                    <template v-if="bucket">
                        <v-chip
                            variant="tonal"
                            filter
                            color="default"
                            class="text-wrap h-100 py-2 mr-2"
                        >
                            {{ Size.toBase10String(bucket.storage * Memory.GB) }}
                        </v-chip>
                        <v-chip
                            variant="tonal"
                            filter
                            color="default"
                            class="text-wrap h-100 py-2"
                        >
                            {{ bucket.objectCount.toLocaleString() }} object{{ (bucket?.objectCount ?? 0) > 1 ? 's' : '' }}
                        </v-chip>
                    </template>
                </p>

                <p class="mt-6 mb-4">Confirm deletion by typing 'DELETE' below:</p>

                <v-text-field
                    id="confirm-delete"
                    v-model="confirmDelete"
                    label="Type DELETE to confirm"
                    outlined
                    dense
                    color="error"
                />

                <v-alert>
                    Bucket deletion runs in your browser session.
                    <strong>Closing this page may interrupt the process.</strong>
                    <br><br>
                    If there are millions of objects, this process may time out.
                    We recommend using one of the approaches mentioned <strong><a href="https://storj.dev/dcs/buckets/delete-buckets" target="_blank" rel="noopener noreferrer">here</a></strong>.
                    <br><br>
                    Object count and statistics might not reflect changes made in the past 24 hours.
                </v-alert>
            </v-card-item>
            <v-card-item v-else class="pa-6">
                <p class="mb-4">
                    The bucket you tried to delete is not empty.
                    You must delete all versions in the bucket.
                </p>

                <v-alert>
                    Please toggle 'show versions', delete all object versions,
                    including delete markers and try again.
                </v-alert>
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block :disabled="isLoading" @click="model = false">
                            {{ hasObjects ? 'Close' : 'Cancel' }}
                        </v-btn>
                    </v-col>
                    <v-col v-if="!hasObjects">
                        <v-btn
                            color="error"
                            variant="flat"
                            block
                            :loading="isLoading"
                            :disabled="confirmDelete?.toUpperCase() !== 'DELETE'"
                            @click="onDelete"
                        >
                            Delete
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
    VAlert,
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VChip,
    VCol,
    VDialog,
    VDivider,
    VRow,
    VSheet,
    VTextField,
} from 'vuetify/components';
import { Trash2, X } from 'lucide-vue-next';

import { Memory, Size } from '@/utils/bytesSize';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { Bucket } from '@/types/buckets';
import { Versioning } from '@/types/versioning';
import { UploadingStatus, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useAccessGrantWorker } from '@/composables/useAccessGrantWorker';

const props = defineProps<{
    bucketName: string;
}>();

const model = defineModel<boolean>({ required: true });

const emit = defineEmits(['deleted']);

const configStore = useConfigStore();
const bucketsStore = useBucketsStore();
const agStore = useAccessGrantsStore();
const projectsStore = useProjectsStore();
const obStore = useObjectBrowserStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();
const { setPermissions, generateAccess } = useAccessGrantWorker();

const confirmDelete = ref<string>();
const hasObjects = ref<boolean>(false);

/**
 * The bucket to be deleted.
 */
const bucket = computed((): Bucket | undefined => {
    return bucketsStore.state.page.buckets.find(b => b.name === props.bucketName);
});

/**
 * Returns API key from store.
 */
const apiKey = computed((): string => {
    return bucketsStore.state.apiKey;
});

async function setCredentials(checkEmpty = false): Promise<void> {
    const projectID = projectsStore.state.selectedProject.id;

    const now = new Date();

    if (!apiKey.value) {
        const name = `${configStore.state.config.objectBrowserKeyNamePrefix}${now.getTime()}`;
        const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(name, projectID);
        bucketsStore.setApiKey(cleanAPIKey.secret);
    }

    const inOneHour = new Date(now.setHours(now.getHours() + 1));

    const macaroon = await setPermissions({
        isDownload: false,
        isUpload: false,
        isList: true,
        isDelete: true,
        notAfter: inOneHour.toISOString(),
        buckets: JSON.stringify([props.bucketName]),
        apiKey: apiKey.value,
    });

    const accessGrant = await generateAccess({
        apiKey: macaroon,
        passphrase: checkEmpty ? bucketsStore.state.passphrase : '',
    }, projectsStore.state.selectedProject.id);

    const edgeCredentials: EdgeCredentials = await agStore.getEdgeCredentials(accessGrant);
    bucketsStore.setEdgeCredentialsForDelete(edgeCredentials, bucket.value?.objectLockEnabled);
}

async function performDelete() {
    const deleteRequest = bucketsStore.deleteBucket(props.bucketName);
    bucketsStore.handleDeleteBucketRequest(props.bucketName, deleteRequest);
    model.value = false;
    emit('deleted');
}

/**
 * Creates unrestricted access grant and deletes bucket
 * when Delete button has been clicked.
 */
async function onDelete(): Promise<void> {
    if (obStore.state.uploading.some(u => u.Bucket === props.bucketName && u.status === UploadingStatus.InProgress)) {
        notify.error('There is an ongoing upload in this bucket.', AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
        return;
    }

    await withLoading(async () => {
        try {
            const checkEmpty = !!(bucket.value?.objectLockEnabled && bucketsStore.state.passphrase);
            await setCredentials(checkEmpty);
            if (!checkEmpty) {
                await performDelete();
                return;
            }
            const isEmpty = await bucketsStore.checkBucketEmpty(bucket.value?.name || '');
            if (isEmpty) {
                await performDelete();
                return;
            }
            hasObjects.value = true;
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
        }
    });
}

watch(model, shown => {
    if (!shown) confirmDelete.value = '';
});
</script>

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
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <icon-trash />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">Delete Bucket</v-card-title>
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

            <div class="pa-6">
                <p class="mb-3">
                    The following bucket and all of its data will be deleted. This action cannot be undone.
                </p>
                <p class="mt-2">
                    <v-chip :title="bucketName" class="font-weight-bold text-wrap h-100 py-2">
                        {{ bucketName }}
                    </v-chip>
                </p>
            </div>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block :disabled="isLoading" @click="model = false">
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="error" variant="flat" block :loading="isLoading" @click="onDelete">
                            Delete
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import {
    VDialog,
    VCard,
    VCardItem,
    VSheet,
    VCardTitle,
    VDivider,
    VCardActions,
    VRow,
    VCol,
    VBtn,
    VChip,
} from 'vuetify/components';

import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';

import IconTrash from '@/components/icons/IconTrash.vue';

const props = defineProps<{
    bucketName: string;
}>();

const model = defineModel<boolean>({ required: true });

const emit = defineEmits(['deleted']);

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const bucketsStore = useBucketsStore();
const agStore = useAccessGrantsStore();
const projectsStore = useProjectsStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const worker = ref<Worker| null>(null);

/**
 * Returns API key from store.
 */
const apiKey = computed((): string => {
    return bucketsStore.state.apiKey;
});

/**
 * Creates unrestricted access grant and deletes bucket
 * when Delete button has been clicked.
 */
async function onDelete(): Promise<void> {
    await withLoading(async () => {
        const projectID = projectsStore.state.selectedProject.id;

        try {
            if (!worker.value) {
                notify.error('Web worker is not initialized.', AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
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
                'isUpload': false,
                'isList': true,
                'isDelete': true,
                'notAfter': inOneHour.toISOString(),
                'buckets': JSON.stringify([props.bucketName]),
                'apiKey': apiKey.value,
            });

            const grantEvent: MessageEvent = await new Promise(resolve => {
                if (worker.value) {
                    worker.value.onmessage = resolve;
                }
            });
            if (grantEvent.data.error) {
                notify.error(grantEvent.data.error, AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
                return;
            }

            const salt = await projectsStore.getProjectSalt(projectsStore.state.selectedProject.id);
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
                notify.error(accessGrantEvent.data.error, AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
                return;
            }

            const accessGrant = accessGrantEvent.data.value;

            const edgeCredentials: EdgeCredentials = await agStore.getEdgeCredentials(accessGrant);
            bucketsStore.setEdgeCredentialsForDelete(edgeCredentials);
            const deleteRequest = bucketsStore.deleteBucket(props.bucketName);
            bucketsStore.handleDeleteBucketRequest(props.bucketName, deleteRequest);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
            return;
        }

        model.value = false;
        emit('deleted');
    });
}

/**
 * Sets local worker with worker instantiated in store.
 */
watch(model, shown => {
    if (!shown) return;
    worker.value = agStore.state.accessGrantsWebWorker;
    if (!worker.value) return;
    worker.value.onerror = (error: ErrorEvent) => {
        notify.error(error.message, AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
    };
});
</script>

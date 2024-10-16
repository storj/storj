// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        max-width="460px"
        transition="fade-transition"
    >
        <v-card :loading="isLoading">
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <component :is="ReceiptText" :size="18" />
                    </v-sheet>
                </template>

                <v-card-title class="font-weight-bold">
                    Bucket Details
                </v-card-title>

                <template #append>
                    <v-btn
                        id="close-bucket-details"
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-card-item>
                <v-list lines="one">
                    <v-list-item title="Name" :subtitle="bucket.name" class="px-0" />
                    <v-list-item title="Objects" :subtitle="bucket.objectCount.toLocaleString()" class="px-0" />
                    <v-list-item title="Segments" :subtitle="bucket.segmentCount.toLocaleString()" class="px-0" />
                    <v-list-item title="Storage" :subtitle="bucket.storage.toFixed(2) + 'GB'" class="px-0" />
                    <v-list-item v-if="showRegionTag" title="Location" :subtitle="bucket.location || `unknown(${bucket.defaultPlacement})`" class="px-0" />
                    <v-list-item v-if="versioningUIEnabled" title="Versioning" :subtitle="bucket.versioning" class="px-0" />
                    <v-list-item v-if="objectLockUIEnabled" title="Object lock" :subtitle="bucket.objectLockEnabled ? 'Enabled' : 'Disabled'" class="px-0" />
                    <v-list-item v-if="objectLockUIEnabled" title="Default Lock Mode" class="px-0">
                        <template #subtitle>
                            <p class="text-capitalize">{{ bucket.defaultRetentionMode.toLowerCase() }}</p>
                        </template>
                    </v-list-item>
                    <v-list-item v-if="objectLockUIEnabled" title="Default Retention Period" :subtitle="defaultRetentionPeriod" class="px-0" />
                    <v-list-item title="Date Created" :subtitle="bucket.since.toUTCString()" class="px-0" />
                    <v-list-item title="Last Updated" :subtitle="bucket.before.toUTCString()" class="px-0" />
                </v-list>
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn color="default" variant="outlined" block @click="model = false">Close</v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
    VDialog,
    VCard,
    VCardItem,
    VCardTitle,
    VDivider,
    VCardActions,
    VRow,
    VCol,
    VBtn,
    VSheet,
    VList,
    VListItem,
} from 'vuetify/components';
import { ReceiptText } from 'lucide-vue-next';

import { Bucket } from '@/types/buckets';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useNotify } from '@/utils/hooks';
import { useLoading } from '@/composables/useLoading';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

const bucketsStore = useBucketsStore();
const configStore = useConfigStore();
const projectsStore = useProjectsStore();
const bucket = ref<Bucket>(new Bucket());
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const props = defineProps<{
    bucketName: string,
}>();

const model = defineModel<boolean>({ required: true });

/**
 * Whether versioning has been enabled for current project.
 */
const versioningUIEnabled = computed(() => projectsStore.versioningUIEnabled);

/**
 * Whether object lock UI is enabled.
 */
const objectLockUIEnabled = computed<boolean>(() => {
    return configStore.objectLockUIEnabled
      && projectsStore.objectLockUIEnabledForProject;
});

const defaultRetentionPeriod = computed(() => {
    const { objectLockEnabled, defaultRetentionDays, defaultRetentionYears } = bucket.value;

    if (!objectLockEnabled) return 'Not Set';

    if (defaultRetentionDays) {
        return `${defaultRetentionDays} Day${defaultRetentionDays > 1 ? 's' : ''}`;
    }

    if (defaultRetentionYears) {
        return `${defaultRetentionYears} Year${defaultRetentionYears > 1 ? 's' : ''}`;
    }

    return 'Not Set';
});

const showRegionTag = computed<boolean>(() => {
    return configStore.state.config.enableRegionTag;
});

/**
 * Fetch the bucket data if it's not available.
 */
async function loadBucketData() {
    if (!projectsStore.state.selectedProject.id) {
        bucket.value = new Bucket();
        return;
    }

    const data = bucketsStore.state.page.buckets.find(
        (bucket: Bucket) => bucket.name === props.bucketName,
    );

    if (data) {
        bucket.value = data;
    } else {
        withLoading(async () => {
            try {
                bucket.value = await bucketsStore.getSingleBucket(projectsStore.state.selectedProject.id, props.bucketName);
            } catch (error) {
                notify.notifyError(error, AnalyticsErrorEventSource.BUCKET_DETAILS_MODAL);
                bucket.value = new Bucket();
            }
        });
    }

}

/**
 *  Load the bucket data when dialog is opened
 */
watch(model, (newValue) => {
    if (newValue) {
        loadBucketData();
    }
});
</script>

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        max-width="460px"
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
                    <v-list-item title="Files" :subtitle="bucket.objectCount.toLocaleString()" class="px-0" />
                    <v-list-item title="Segments" :subtitle="bucket.segmentCount.toLocaleString()" class="px-0" />
                    <v-list-item title="Storage" :subtitle="bucket.storage.toFixed(2) + 'GB'" class="px-0" />
                    <v-list-item v-if="showRegionTag" title="Location" :subtitle="bucket.location || `unknown(${bucket.defaultPlacement})`" class="px-0" />
                    <v-list-item v-if="versioningUIEnabled" title="Versioning" :subtitle="bucket.versioning" class="px-0" />
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
import { computed } from 'vue';
import { useRouter } from 'vue-router';
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
import { ROUTES } from '@/router';
import { useConfigStore } from '@/store/modules/configStore';

const bucketsStore = useBucketsStore();
const configStore = useConfigStore();
const projectsStore = useProjectsStore();
const router = useRouter();

const props = defineProps<{
    bucketName: string,
}>();

const model = defineModel<boolean>({ required: true });

/**
 * Whether versioning has been enabled for current project.
 */
const versioningUIEnabled = computed(() => projectsStore.versioningUIEnabled);

const showRegionTag = computed<boolean>(() => {
    return configStore.state.config.enableRegionTag;
});

function redirectToBucketsPage(): void {
    router.push({
        name: ROUTES.Buckets.name,
        params: { id: projectsStore.state.selectedProject.urlId },
    });
}

const bucket = computed((): Bucket => {
    if (!projectsStore.state.selectedProject.id) return new Bucket();

    const data = bucketsStore.state.page.buckets.find(
        (bucket: Bucket) => bucket.name === props.bucketName,
    );

    if (!data) {
        redirectToBucketsPage();

        return new Bucket();
    }

    return data;
});
</script>

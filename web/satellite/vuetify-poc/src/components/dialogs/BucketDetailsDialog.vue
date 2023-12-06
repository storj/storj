// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        min-width="320px"
        max-width="460px"
        transition="fade-transition"
    >
        <v-card rounded="xlg">
            <v-card-item class="pa-5 pl-7">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <icon-bucket />
                    </v-sheet>
                </template>

                <v-card-title class="font-weight-bold">
                    Bucket Details
                </v-card-title>

                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-card-item class="pl-7">
                <h4>Name</h4>
                <p>{{ bucket.name }}</p>
            </v-card-item>

            <v-card-item class="pl-7">
                <h4>Files</h4>
                <p>{{ bucket.objectCount.toLocaleString() }}</p>
            </v-card-item>

            <v-card-item class="pl-7">
                <h4>Segments</h4>
                <p>{{ bucket.segmentCount.toLocaleString() }}</p>
            </v-card-item>

            <v-card-item class="pl-7">
                <h4>Date Created</h4>
                <p>{{ bucket.since.toUTCString() }}</p>
            </v-card-item>

            <v-card-item class="mb-4 pl-7">
                <h4>Last Updated</h4>
                <p>{{ bucket.before.toUTCString() }}</p>
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-7">
                <v-row>
                    <v-col>
                        <v-btn color="primary" variant="flat" block @click="model = false">Close</v-btn>
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
} from 'vuetify/components';

import IconBucket from '../icons/IconBucket.vue';

import { useLoading } from '@/composables/useLoading';
import { Bucket } from '@/types/buckets';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

const isLoading = useLoading();
const bucketsStore = useBucketsStore();
const projectsStore = useProjectsStore();
const router = useRouter();
const emit = defineEmits<{
    (event: 'update:modelValue', value: boolean): void,
}>();
const props = defineProps<{
    modelValue: boolean,
    bucketName: string,
}>();
const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

function redirectToBucketsPage(): void {
    router.push(`/projects/${projectsStore.state.selectedProject.urlId}/buckets`);
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
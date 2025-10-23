// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        max-width="450px"
        transition="fade-transition"
    >
        <v-card ref="innerContent" rounded="xlg">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center text-error"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <component :is="Lock" :size="18" />
                        </v-sheet>
                    </template>
                    <v-card-title class="font-weight-bold">
                        Can't Delete Locked Version
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

            <v-row>
                <v-col class="pa-6 mx-3">
                    <p class="mt-3 mb-1 font-weight-bold text-body-2">
                        Name:
                    </p>
                    <v-chip
                        variant="tonal"
                        filter
                        color="default"
                    >
                        {{ file?.Key }}
                    </v-chip>

                    <template v-if="file?.VersionId">
                        <p class="mt-3 mb-1 font-weight-bold text-body-2">
                            Version:
                        </p>
                        <v-chip
                            variant="tonal"
                            filter
                            color="default"
                        >
                            {{ file?.VersionId }}
                        </v-chip>
                    </template>

                    <template v-if="file?.legalHold">
                        <p class="mt-3 mb-1 font-weight-bold text-body-2">
                            Legal Hold:
                        </p>

                        <v-chip
                            variant="tonal"
                            filter
                            color="error"
                        >
                            Yes
                        </v-chip>
                    </template>

                    <template v-if="file?.retention?.active">
                        <p class="mt-3 mb-1 font-weight-bold text-body-2">
                            Lock Mode:
                        </p>

                        <v-chip
                            variant="tonal"
                            filter
                            color="error"
                        >
                            {{ file.retention.mode.substring(0, 1) + file.retention.mode.substring(1).toLowerCase() }}
                        </v-chip>

                        <p class="mt-3 mb-1 font-weight-bold text-body-2">
                            Locked until:
                        </p>

                        <v-chip
                            variant="tonal"
                            filter
                            color="error"
                        >
                            {{ formatDate(file?.retention?.retainUntil) }}
                        </v-chip>
                    </template>

                    <v-alert class="my-4" type="error" variant="outlined" border>
                        This version of the object is currently locked and cannot be deleted.
                        Locking prevents accidental or unauthorized changes to important data.
                    </v-alert>
                </v-col>
            </v-row>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block @click="model = false">
                            Close
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="primary" variant="flat" block @click="goToDocs">
                            Learn More
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
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
} from 'vuetify/components';
import { Lock, X } from 'lucide-vue-next';

import { Time } from '@/utils/time';
import { FullBrowserObject } from '@/store/modules/objectBrowserStore';
import {
    AnalyticsEvent,
    PageVisitSource,
} from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

const analyticsStore = useAnalyticsStore();

defineProps<{
    file: FullBrowserObject | null,
}>();

const model = defineModel<boolean>();

const emit = defineEmits<{
    'contentRemoved': [],
}>();

const innerContent = ref<VCard | null>(null);

function formatDate(date?: Date): string {
    if (!date) {
        return '-';
    }
    return Time.formattedDate(date, { day: 'numeric', month: 'long', year: 'numeric' });
}

function goToDocs() {
    analyticsStore.pageVisit('https://storj.dev/dcs/api/s3/object-lock', PageVisitSource.DOCS);
    analyticsStore.eventTriggered(AnalyticsEvent.VIEW_DOCS_CLICKED);
    window.open('https://storj.dev/dcs/api/s3/object-lock', '_blank', 'noreferrer');
}

watch(innerContent, comp => !comp && emit('contentRemoved'));
</script>

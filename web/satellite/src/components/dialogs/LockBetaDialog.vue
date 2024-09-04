// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        activator="parent"
        width="auto"
        min-width="400px"
        max-width="450px"
        transition="fade-transition"
    >
        <v-card rounded="xlg">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <icon-lock :size="20" />
                        </v-sheet>
                    </template>
                    <v-card-title class="font-weight-bold">
                        Object Lock (Beta)
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
            </v-sheet>

            <v-divider />

            <div class="pa-6">
                <v-row>
                    <v-col>
                        <p>
                            Object Lock, enabled through Object Versioning, allows you to lock individual files from being deleted or overwritten for a specified period of time.
                        </p>

                        <v-expansion-panels static>
                            <v-expansion-panel
                                title="How it works"
                                elevation="0"
                                rounded="lg"
                                class="border my-4 font-weight-bold"
                                static
                            >
                                <v-expansion-panel-text class="text-body-2">
                                    <p class="my-2">Object Lock is available for buckets with Object Versioning enabled.</p>
                                    <p class="my-2">A new column displaying the lock status will appear in your buckets page.</p>
                                    <p class="my-2">When object lock is enabled, you can lock files and choose the retention period.</p>
                                    <p class="my-2">You can view locked files and their retention period in the browser.</p>
                                </v-expansion-panel-text>
                            </v-expansion-panel>
                        </v-expansion-panels>
                        <v-expansion-panels static>
                            <v-expansion-panel
                                title="How to use it"
                                elevation="0"
                                rounded="lg"
                                class="border mb-6 font-weight-bold"
                                static
                            >
                                <v-expansion-panel-text class="text-body-2">
                                    <p class="my-2">1. Ensure Object Versioning is enabled for your bucket. Then, enable Object Lock for that versioned bucket.</p>
                                    <p class="my-2">2. Upload files to your versioned bucket and lock them as needed. Each change will create a new version of the file.</p>
                                    <p class="my-2">3. Once locked, files cannot be deleted or overwritten until the retention period expires.</p>
                                </v-expansion-panel-text>
                            </v-expansion-panel>
                        </v-expansion-panels>

                        <p class="text-body-2">
                            For more information, <a href="" class="link" @click="goToDocs">visit the documentation</a>.
                        </p>
                    </v-col>
                </v-row>
            </div>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            @click="model = false"
                        >
                            Close
                        </v-btn>
                    </v-col>

                    <v-col>
                        <v-btn
                            variant="flat"
                            block
                            @click="goToDocs"
                        >
                            Read Documentation
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VCol,
    VDialog,
    VDivider,
    VExpansionPanel,
    VExpansionPanels,
    VExpansionPanelText,
    VRow,
    VSheet,
} from 'vuetify/components';

import { AnalyticsEvent, PageVisitSource } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import IconLock from '@/components/icons/IconLock.vue';

const analyticsStore = useAnalyticsStore();

const model = defineModel<boolean>({ default: false });

function goToDocs() {
    analyticsStore.pageVisit('https://docs.storj.io/dcs/buckets/object-versioning', PageVisitSource.DOCS);
    analyticsStore.eventTriggered(AnalyticsEvent.VIEW_DOCS_CLICKED);
    window.open('https://docs.storj.io/dcs/buckets/object-versioning', '_blank', 'noreferrer');
}
</script>

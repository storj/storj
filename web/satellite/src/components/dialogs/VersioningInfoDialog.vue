// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="isDialogOpen"
        activator="parent"
        width="auto"
        max-width="450px"
        transition="fade-transition"
    >
        <v-card>
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <component :is="History" :size="18" />
                        </v-sheet>
                    </template>
                    <v-card-title class="font-weight-bold">
                        Object Versioning
                    </v-card-title>
                    <template #append>
                        <v-btn
                            :icon="X"
                            variant="text"
                            size="small"
                            color="default"
                            @click="isDialogOpen = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <div class="pa-6">
                <v-row>
                    <v-col>
                        <p>
                            Versioning is enabled for this project. Learn how it works.
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
                                    <p class="my-2">Versioning can be activated for each bucket individually.</p>
                                    <p class="my-2">A new column displaying the versioning status will appear on your buckets page.</p>
                                    <p class="my-2">When versioning is enabled, each object in the bucket will have a unique version ID.</p>
                                    <p class="my-2">You can easily retrieve, list, and restore previous versions of your objects.</p>
                                    <p v-if="objectLockEnabled" class="my-2">Object Lock can be applied to versioned objects for additional protection.</p>
                                </v-expansion-panel-text>
                            </v-expansion-panel>
                        </v-expansion-panels>
                        <v-expansion-panels static>
                            <v-expansion-panel
                                title="Next steps"
                                elevation="0"
                                rounded="lg"
                                class="border mb-6 font-weight-bold"
                                static
                            >
                                <v-expansion-panel-text class="text-body-2">
                                    <p class="my-2">1. Create a new bucket with versioning enabled from the start, or enable versioning on existing buckets that support it.</p>
                                    <p class="my-2">2. Upload objects to your versioned bucket and make changes as needed. Each change will create a new version of the object.</p>
                                    <p class="my-2">3. Use the version ID to retrieve, list, or restore specific versions of your objects.</p>
                                    <p v-if="objectLockEnabled" class="my-2">4. Protect your objects from deletion or modification by applying Object Lock to versioned objects.</p>
                                </v-expansion-panel-text>
                            </v-expansion-panel>
                        </v-expansion-panels>

                        <p class="text-body-2">
                            For more information, <a
                                :href="docsLink"
                                class="link"
                                target="_blank"
                                rel="noopener noreferrer"
                                @click="trackGoToDocs"
                            >visit the documentation</a>.
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
                            @click="isDialogOpen = false"
                        >
                            Close
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            variant="flat"
                            color="primary"
                            block
                            :href="docsLink"
                            target="_blank"
                            @click="trackGoToDocs"
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
import { History, X } from 'lucide-vue-next';
import { computed } from 'vue';

import { AnalyticsEvent, PageVisitSource } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useConfigStore } from '@/store/modules/configStore';

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();

const isDialogOpen = defineModel<boolean>({ default: false });

const docsLink = 'https://storj.dev/dcs/api/s3/object-versioning';

/**
 * whether object lock UI is globally enabled.
 */
const objectLockEnabled = computed<boolean>(() => configStore.state.config.objectLockUIEnabled);

function trackGoToDocs(): void {
    analyticsStore.pageVisit(docsLink, PageVisitSource.DOCS);
    analyticsStore.eventTriggered(AnalyticsEvent.VIEW_DOCS_CLICKED);
}
</script>

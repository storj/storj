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
                        Object Versioning (Beta)
                    </v-card-title>
                    <template #append>
                        <v-btn
                            icon="$close"
                            variant="text"
                            size="small"
                            color="default"
                            @click="isDialogOpen = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-window v-model="step">
                <v-window-item :value="0">
                    <div class="pa-6">
                        <v-row>
                            <v-col>
                                <p>
                                    Versioning allows you to preserve, retrieve, and restore previous versions of a file, offering protection against unintentional modifications or deletions.
                                </p>
                                <v-alert color="default" variant="tonal" class="my-4">
                                    <v-alert-title class="text-body-2">Beta Information</v-alert-title>
                                    Object versioning is in beta, and we're counting on your feedback to perfect it. If you encounter any issues, please tell us about it.
                                </v-alert>
                                <v-checkbox v-model="optedIn" density="compact" class="mt-2 mb-1" label="I understand, and I want to try versioning." hide-details="auto" />
                            </v-col>
                        </v-row>
                    </div>
                </v-window-item>

                <v-window-item :value="1">
                    <div class="pa-6">
                        <v-row>
                            <v-col>
                                <p>
                                    Versioning has been successfully enabled for this project. Learn how it works, and see the next steps.
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
                                        </v-expansion-panel-text>
                                    </v-expansion-panel>
                                </v-expansion-panels>

                                <p class="text-body-2">
                                    For more information, <a
                                        href="https://docs.storj.io/dcs/buckets/object-versioning"
                                        class="link"
                                        target="_blank"
                                        @click="() => trackViewDocsEvent('https://docs.storj.io/')"
                                    >visit the documentation</a>.
                                </p>
                            </v-col>
                        </v-row>
                    </div>
                </v-window-item>
            </v-window>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col v-if="step === 0">
                        <v-btn
                            variant="outlined"
                            color="default"
                            :disabled="isLoading"
                            block
                            @click="isDialogOpen = false"
                        >
                            {{ step === 0 ? 'Cancel' : 'Close' }}
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary"
                            variant="flat"
                            :disabled="!optedIn"
                            :loading="isLoading"
                            block
                            @click="optInOrOut"
                        >
                            <template v-if="info">
                                Close
                            </template>
                            <template v-else>
                                {{ step === 0 ? 'Enable Versioning' : 'Finish' }}
                            </template>
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import {
    VAlert,
    VAlertTitle,
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VCheckbox,
    VCol,
    VDialog,
    VDivider,
    VExpansionPanel,
    VExpansionPanels,
    VExpansionPanelText,
    VRow,
    VSheet,
    VWindow,
    VWindowItem,
} from 'vuetify/components';
import { History } from 'lucide-vue-next';
import { ref, watchEffect } from 'vue';

import { useLoading } from '@/composables/useLoading';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource, PageVisitSource } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

const analyticsStore = useAnalyticsStore();
const projectStore = useProjectsStore();

const notify = useNotify();
const { withLoading, isLoading } = useLoading();

const isDialogOpen = defineModel<boolean>();

const props = defineProps<{
    info?: boolean;
}>();

const optedIn = ref(false);
const step = ref(0);

function trackViewDocsEvent(link: string): void {
    analyticsStore.pageVisit(link, PageVisitSource.DOCS);
}

function optInOrOut() {
    if (step.value === 1) {
        isDialogOpen.value = false;
        return;
    }
    const inOrOut = optedIn.value ? 'in' : 'out';
    withLoading(async () => {
        try {
            await projectStore.setVersioningOptInStatus(inOrOut);
            await projectStore.getProjectConfig();
            projectStore.getProjects();

            step.value++;
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.VERSIONING_BETA_DIALOG);
        }
    });
}

watchEffect(() => {
    if (isDialogOpen.value && props.info) {
        step.value = 1;
        optedIn.value = true;
    }
});
</script>
// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="isDialogOpen"
        activator="parent"
        width="auto"
        min-width="400px"
        max-width="450px"
        transition="fade-transition"
    >
        <v-card>
            <v-sheet>
                <v-card-item class="py-4 pl-6">
                    <template #prepend>
                        <icon-versioning-clock size="40" />
                    </template>
                    <v-card-title class="font-weight-bold">
                        Try Object Versioning (Beta)
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

            <v-row>
                <v-col class="pa-6 mx-3">
                    <p class="my-2">
                        Versioning allows you to preserve, retrieve, and restore previous versions of a file, offering protection against unintentional modifications or deletions.
                    </p>
                    <v-alert color="info" variant="tonal" class="my-4">
                        By activating it, you can enable versioning per bucket, and keep multiple versions of each file.
                    </v-alert>
                    <v-alert color="warning" variant="tonal" class="my-4">
                        Object versioning is in beta, and we're counting on your feedback to perfect it. If you encounter any issues, please tell us about it.
                    </v-alert>
                    <v-checkbox v-model="optedIn" density="compact" class="mt-2 mb-1" label="I understand, and I want to try versioning." hide-details="auto" />
                </v-col>
            </v-row>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            :disabled="isLoading"
                            block
                            @click="isDialogOpen = false"
                        >
                            Cancel
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
                            Enable Versioning
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
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VCheckbox,
    VCol,
    VDialog,
    VDivider,
    VRow,
    VSheet,
} from 'vuetify/components';
import { ref } from 'vue';

import { useLoading } from '@/composables/useLoading';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import IconVersioningClock from '@/components/icons/IconVersioningClock.vue';

const projectStore = useProjectsStore();

const notify = useNotify();
const { withLoading, isLoading } = useLoading();

const optedIn = ref(false);
const isDialogOpen = ref(false);

function optInOrOut() {
    const inOrOut = optedIn.value ? 'in' : 'out';
    withLoading(async () => {
        try {
            await projectStore.setVersioningOptInStatus(inOrOut);
            await projectStore.getProjectConfig();
            projectStore.getProjects();

            isDialogOpen.value = false;
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.VERSIONING_BETA_DIALOG);
        }
    });
}
</script>
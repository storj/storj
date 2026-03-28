// Copyright (C) 2026 Storj Labs, Inc.
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
                        <component :is="Bell" :size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">Limit Notifications</v-card-title>
                <template #append>
                    <v-btn
                        :icon="X"
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
                <v-row class="mb-2 align-center" no-gutters>
                    <v-col>
                        <p class="font-weight-bold text-body-2">Storage notifications</p>
                        <p class="text-body-2 text-medium-emphasis">Receive emails when storage usage reaches 80% and 100% of the project limit.</p>
                    </v-col>
                    <v-col cols="auto" class="ml-4">
                        <v-switch
                            v-model="storageEnabled"
                            hide-details
                            inset
                            density="compact"
                            color="primary"
                        />
                    </v-col>
                </v-row>

                <v-divider class="my-4" />

                <v-row class="align-center" no-gutters>
                    <v-col>
                        <p class="font-weight-bold text-body-2">Egress notifications</p>
                        <p class="text-body-2 text-medium-emphasis">Receive emails when download usage reaches 80% and 100% of the project limit.</p>
                    </v-col>
                    <v-col cols="auto" class="ml-4">
                        <v-switch
                            v-model="egressEnabled"
                            hide-details
                            inset
                            density="compact"
                            color="primary"
                        />
                    </v-col>
                </v-row>
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
                        <v-btn color="primary" variant="flat" block :loading="isLoading" @click="onSaveClick">
                            Save
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
    VSwitch,
} from 'vuetify/components';
import { Bell, X } from 'lucide-vue-next';

import { useLoading } from '@/composables/useLoading';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

const model = defineModel<boolean>({ required: true });

const projectsStore = useProjectsStore();
const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const storageEnabled = ref<boolean>(false);
const egressEnabled = ref<boolean>(false);

function onSaveClick(): void {
    withLoading(async () => {
        try {
            await projectsStore.updateLimitNotifications({
                storageNotificationsEnabled: storageEnabled.value,
                egressNotificationsEnabled: egressEnabled.value,
            });
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_SETTINGS_AREA);
            return;
        }

        notify.success('Notification preferences updated.');
        model.value = false;
    });
}

watch(model, shown => {
    if (!shown) return;
    const project = projectsStore.state.selectedProject;
    storageEnabled.value = project.storageLimitNotificationsEnabled;
    egressEnabled.value = project.egressLimitNotificationsEnabled;
}, { immediate: true });
</script>

// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        scrollable
        max-width="400px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card rounded="xlg" :loading="isLoading">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <component :is="Computer" :size="18" />
                        </v-sheet>
                    </template>

                    <v-card-title class="font-weight-bold">
                        Stop Instance
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

            <v-card-item class="px-6">
                <p>
                    Are you sure you want to stop the instance
                    <strong>{{ instance.name }}</strong>?
                </p>
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            :disabled="isLoading"
                            @click="model = false"
                        >
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary"
                            variant="flat"
                            block
                            :loading="isLoading"
                            @click="stopInstance"
                        >
                            Stop
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
    VRow,
    VSheet,
} from 'vuetify/components';
import { Computer, X } from 'lucide-vue-next';

import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { useComputeStore } from '@/store/modules/computeStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { Instance } from '@/types/compute';

const computeStore = useComputeStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const props = defineProps<{
    instance: Instance
}>();

const model = defineModel<boolean>({ required: true });

function stopInstance(): void {
    withLoading(async () => {
        try {
            await computeStore.stopInstance(props.instance.id);

            notify.success('Instance stopped successfully');
            model.value = false;
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.STOP_OR_RESTART_COMPUTE_INSTANCE_DIALOG);
        }
    });
}
</script>

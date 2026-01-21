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

                    <v-card-title class="font-weight-bold text-capitalize">
                        {{ action }} Instance
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
                    Are you sure you want to {{ action }} the instance
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
                            class="text-capitalize"
                            :loading="isLoading"
                            @click="action === InstanceAction.STOP ? stopInstance() : restartInstance()"
                        >
                            {{ action }}
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
import { onBeforeUnmount, ref } from 'vue';

import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { useComputeStore } from '@/store/modules/computeStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { Instance, InstanceAction } from '@/types/compute';

const computeStore = useComputeStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const props = defineProps<{
    action: InstanceAction;
    instance: Instance;
}>();

const model = defineModel<boolean>({ required: true });

const pollingInterval = ref<NodeJS.Timeout>();

function stopInstance(): void {
    withLoading(async () => {
        try {
            await computeStore.stopInstance(props.instance.id);
            notify.success('Instance stop initiated');
            await computeStore.getInstance(props.instance.id);

            model.value = false;
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.STOP_OR_RESTART_COMPUTE_INSTANCE_DIALOG);
        }
    });
}

function restartInstance(): void {
    clearPollingInterval();

    withLoading(async () => {
        const instanceId = props.instance.id;

        try {
            await computeStore.stopInstance(instanceId);
            notify.success('Instance restart initiated');

            // Poll until the instance is stopped (or timeout).
            const timeoutMs = 2 * 60 * 1000; // 2 minutes.
            const pollEveryMs = 5000;

            const startedAt = Date.now();

            await new Promise<void>((resolve, reject) => {
                const tick = async () => {
                    if (Date.now() - startedAt > timeoutMs) {
                        clearPollingInterval();

                        reject(new Error('Timed out waiting for the instance to stop'));
                        return;
                    }

                    try {
                        const instance = await computeStore.getInstance(instanceId);
                        if (!instance.running) {
                            clearPollingInterval();

                            resolve();
                        }
                    } catch (error) {
                        clearPollingInterval();

                        reject(error);
                    }
                };

                void tick();

                pollingInterval.value = setInterval(() => {
                    void tick();
                }, pollEveryMs);
            });

            await computeStore.startInstance(instanceId);
            notify.success('Instance restart succeeded');
            await computeStore.getInstance(props.instance.id);

            model.value = false;
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.STOP_OR_RESTART_COMPUTE_INSTANCE_DIALOG);
        } finally {
            clearPollingInterval();
        }
    });
}

function clearPollingInterval(): void {
    if (pollingInterval.value) {
        clearInterval(pollingInterval.value);
        pollingInterval.value = undefined;
    }
}

onBeforeUnmount(() => {
    clearPollingInterval();
});
</script>

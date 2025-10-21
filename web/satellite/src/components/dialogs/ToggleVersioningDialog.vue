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
                        <icon-versioning-clock size="40" />
                    </template>
                    <v-card-title class="font-weight-bold">
                        {{ isSuspending ? 'Suspend' : 'Enable' }} Versioning
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

            <v-row>
                <v-col class="pa-6 mx-3">
                    <p class="my-2">
                        Do you want to {{ isSuspending ? 'suspend' : 'enable' }} versioning on this bucket?
                    </p>
                    <v-alert color="info" variant="tonal" class="my-4">
                        {{
                            isSuspending
                                ? 'By suspending versioning, uploading objects with the same name will overwrite them. Previously stored versions will remain accessible.'
                                : 'By enabling versioning, you can store multiple versions of each object. All versions count as additional storage used in this project.'
                        }}
                    </v-alert>
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
                            :loading="isLoading"
                            block @click="onToggleVersioning"
                        >
                            {{ isSuspending ? 'Suspend' : 'Enable' }} Versioning
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
    VCol,
    VDialog,
    VDivider,
    VRow,
    VSheet,
} from 'vuetify/components';
import { computed } from 'vue';
import { X } from 'lucide-vue-next';

import { BucketMetadata } from '@/types/buckets';
import { Versioning } from '@/types/versioning';
import { useVersioning } from '@/composables/useVersioning';
import { useLoading } from '@/composables/useLoading';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';

import IconVersioningClock from '@/components/icons/IconVersioningClock.vue';

const { toggleVersioning } = useVersioning();
const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const model = defineModel<BucketMetadata | null>({ required: true });

const emit = defineEmits<{
    'toggle': [];
}>();

const isDialogOpen = computed({
    get: () => !!model.value,
    set: (value: boolean) => {
        if (!value) {
            model.value = null;
        }
    },
});

const isSuspending = computed(() => model.value?.versioning === Versioning.Enabled);

/**
 * Toggles versioning for the bucket between Suspended and Enabled.
 */
function onToggleVersioning() {
    withLoading(async () => {
        if (!model.value) {
            return;
        }
        try {
            await toggleVersioning(model.value.name, model.value.versioning);
            notify.success(`Versioning has been ${model.value.versioning !== Versioning.Enabled ? 'enabled' : 'suspended'} for this bucket.`);
            model.value = null;
            emit('toggle');
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.VERSIONING_TOGGLE_DIALOG);
        }
    });
}
</script>
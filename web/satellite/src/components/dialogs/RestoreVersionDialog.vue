// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        min-width="400px"
        max-width="450px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card ref="innerContent">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <icon-restore />
                        </v-sheet>
                    </template>
                    <v-card-title class="font-weight-bold">
                        Restore Previous Version
                    </v-card-title>
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
            </v-sheet>

            <v-divider />

            <v-card-item v-if="file">
                <p class="mt-2 mb-3">
                    You are about to restore a previous version. This action will create a copy as the latest version of the object. All existing versions, including the current one, will be preserved.
                </p>
                <p class="my-1">
                    Name:
                </p>
                <v-chip class="font-weight-bold text-wrap py-2 mb-3">
                    {{ file.Key }}
                </v-chip>

                <p class="my-1">
                    Version ID:
                </p>
                <v-chip class="font-weight-bold text-wrap py-2 mb-3">
                    {{ file.VersionId }}
                </v-chip>

                <p class="my-1">
                    Date:
                </p>
                <v-chip class="font-weight-bold text-wrap py-2 mb-3">
                    {{ Time.formattedDate(file.LastModified) }}
                </v-chip>
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block :disabled="isLoading" @click="model = false">
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="primary" variant="flat" block :loading="isLoading" @click="onRestoreObjectClick">
                            Restore Version
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref, Component, watch } from 'vue';
import {
    VDialog,
    VCard,
    VCardItem,
    VSheet,
    VCardTitle,
    VDivider,
    VCardActions,
    VRow,
    VCol,
    VBtn,
    VChip,
} from 'vuetify/components';
import { X } from 'lucide-vue-next';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { Time } from '@/utils/time';
import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';

import IconRestore from '@/components/icons/IconRestore.vue';

const props = defineProps<{
    file?: BrowserObject,
}>();

const emit = defineEmits<{
    'contentRemoved': [],
    'fileRestored': [],
}>();

const model = defineModel<boolean>({ required: true });

const obStore = useObjectBrowserStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const innerContent = ref<Component | null>(null);

async function onRestoreObjectClick(): Promise<void> {
    await withLoading(async () => {
        if (!props.file) {
            return;
        }
        try {
            await obStore.restoreObject(props.file);
        } catch (error) {
            error.message = `Error restoring previous version. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.FILE_BROWSER);
            return;
        }
        emit('fileRestored');
        notify.success(`Previous version restored.`);
        model.value = false;
    });
}

watch(innerContent, comp => !comp && emit('contentRemoved'));
</script>
// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="410px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card rounded="xlg" ref="innerContent">
            <v-card-item class="pl-7 py-4">
                <template #prepend>
                    <v-sheet
                        class="bg-on-surface-variant d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <icon-trash />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">Delete Folder</v-card-title>
                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <div class="pa-7">
                The following folder and all of its data will be deleted. This action cannot be undone.
                <br><br>
                <span class="font-weight-bold">{{ folder.Key }}</span>
            </div>

            <v-divider />

            <v-card-actions class="pa-7">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block :disabled="isLoading" @click="model = false">
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="error" variant="flat" block :loading="isLoading" @click="onDeleteClick">
                            Delete
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, Component, watch } from 'vue';
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
} from 'vuetify/components';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';

import IconTrash from '@poc/components/icons/IconTrash.vue';

const props = defineProps<{
    modelValue: boolean,
    folder: BrowserObject,
}>();

const emit = defineEmits<{
    'update:modelValue': [value: boolean],
    'contentRemoved': [],
}>();

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

const obStore = useObjectBrowserStore();
const bucketsStore = useBucketsStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const innerContent = ref<Component | null>(null);

const filePath = computed<string>(() => bucketsStore.state.fileComponentPath);

async function onDeleteClick(): Promise<void> {
    await withLoading(async () => {
        try {
            await obStore.deleteFolder(props.folder, filePath.value ? filePath.value + '/' : '');
        } catch (error) {
            error.message = `Error deleting folder. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.FILE_BROWSER_ENTRY);
            return;
        }

        notify.success('Folder deleted.');
        model.value = false;
    });
}

watch(innerContent, comp => !comp && emit('contentRemoved'));
</script>

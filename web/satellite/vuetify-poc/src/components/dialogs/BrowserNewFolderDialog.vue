// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="dialog"
        activator="parent"
        width="auto"
        min-width="400px"
        transition="fade-transition"
    >
        <v-card ref="innerContent" rounded="xlg">
            <v-sheet>
                <v-card-item class="pl-7 py-4">
                    <template #prepend>
                        <v-card-title class="font-weight-bold">
                            New Folder
                        </v-card-title>
                    </template>

                    <template #append>
                        <v-btn
                            icon="$close"
                            variant="text"
                            size="small"
                            color="default"
                            @click="dialog = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-form v-model="formValid" class="pa-8 pb-3">
                <v-row>
                    <v-col
                        cols="12"
                    >
                        <v-text-field
                            v-model="folder"
                            variant="outlined"
                            :rules="folderRules"
                            label="Enter Folder Name"
                            :hide-details="false"
                            required
                            autofocus
                        />
                    </v-col>
                </v-row>
            </v-form>

            <v-divider />

            <v-card-actions class="pa-7">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block @click="dialog = false">Cancel</v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="primary" variant="flat" :disabled="!formValid" block @click="createFolder">Create Folder</v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { Component, computed, ref, watch } from 'vue';
import {
    VDialog,
    VCard,
    VCardTitle,
    VBtn,
    VSheet,
    VCardItem,
    VForm,
    VRow,
    VCol,
    VTextField,
    VDivider,
    VCardActions,
} from 'vuetify/components';

import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useAppStore } from '@/store/modules/appStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useLoading } from '@/composables/useLoading';

const obStore = useObjectBrowserStore();
const appStore = useAppStore();
const notify = useNotify();

const { isLoading, withLoading } = useLoading();

const dialog = ref<boolean>(false);
const formValid = ref<boolean>(false);
const folder = ref<string>('');
const innerContent = ref<Component | null>(null);

const folderRules = [
    (value: string) => (!!value || 'Folder name is required.'),
    (value: string) => (value.trim().length > 0 || 'Name must not be only space.'),
    (value: string) => (value.indexOf('/') === -1 || 'Name must not contain "/".'),
    (value: string) => ([...value.trim()].filter(c => c === '.').length !== value.trim().length || 'Name must not be only periods.'),
    (value: string) => (files.value.filter(f => f.Key === value.trim()).length === 0 || 'This folder already exists.'),
];

/**
 * Retrieve all the files sorted from the store.
 */
const files = computed((): BrowserObject[] => {
    return obStore.sortedFiles;
});

function createFolder(): void {
    withLoading(async () => {
        try {
            await obStore.createFolder(folder.value.trim());
        } catch (error) {
            notify.error(error.message, AnalyticsErrorEventSource.CREATE_FOLDER_MODAL);
        }
        dialog.value = false;
    });
}

watch(innerContent, comp => {
    if (!comp) {
        folder.value = '';
        return;
    }
});
</script>

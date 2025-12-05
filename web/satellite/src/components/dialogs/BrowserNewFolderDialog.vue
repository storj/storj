// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="420px"
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
                            <img src="@/assets/icon-folder.svg" alt="folder icon">
                        </v-sheet>
                    </template>

                    <v-card-title class="font-weight-bold">
                        New Folder
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

            <v-form v-model="formValid" class="px-6 pt-9 pb-4" @submit.prevent="createFolder">
                <v-row>
                    <v-col cols="12">
                        <v-text-field
                            id="Folder Name"
                            v-model="folder"
                            variant="outlined"
                            :rules="folderRules"
                            label="Folder Name"
                            placeholder="Enter a folder name"
                            :hide-details="false"
                            maxlength="50"
                            required
                            autofocus
                        />
                    </v-col>
                </v-row>
            </v-form>

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
                            :disabled="!formValid"
                            :loading="isLoading"
                            block
                            @click="createFolder"
                        >
                            Create Folder
                        </v-btn>
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
import { X } from 'lucide-vue-next';

import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useLoading } from '@/composables/useLoading';

const obStore = useObjectBrowserStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const model = defineModel<boolean>({ required: true });

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
    if (!formValid.value) return;

    withLoading(async () => {
        try {
            await obStore.createFolder(folder.value.trim());
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.CREATE_FOLDER_MODAL);
        }
        model.value = false;
    });
}

watch(innerContent, comp => {
    if (!comp) {
        folder.value = '';
        return;
    }
});
</script>

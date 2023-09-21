// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="400px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card rounded="xlg">
            <v-card-item class="pl-7 py-4 pos-relative">
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
                <v-card-title class="font-weight-bold">
                    Delete Access
                </v-card-title>
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
                The following access will be deleted. Linksharing URLs using this access will no longer work.
                <br><br>
                <span class="font-weight-bold">{{ accessName }}</span>
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
import { computed } from 'vue';
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

import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import IconTrash from '@poc/components/icons/IconTrash.vue';

const agStore = useAccessGrantsStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const props = defineProps<{
    modelValue: boolean;
    accessName: string;
}>();

const emit = defineEmits<{
    'update:modelValue': [value: boolean],
    'deleted': [];
}>();

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

async function onDeleteClick(): Promise<void> {
    await withLoading(async () => {
        try {
            await agStore.deleteAccessGrantByNameAndProjectID(props.accessName, projectsStore.state.selectedProject.id);
            emit('deleted');
            model.value = false;
        } catch (error) {
            error.message = `Error deleting access grant. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.CONFIRM_DELETE_AG_MODAL);
        }
    });
}
</script>

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
            <v-card-item class="pa-5 pl-7">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <icon-trash />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">
                    Delete Access Key{{ accessNames.length > 1 ? 's' : '' }}
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
                The following access key{{ accessNames.length > 1 ? 's' : '' }}
                will be deleted. Any publicly shared links using
                {{ accessNames.length > 1 ? 'these access keys' : 'this access key' }} will no longer work.
                <br><br>
                <p v-for="accessName of accessNames" :key="accessName" class="font-weight-bold">{{ accessName }}</p>
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

import IconTrash from '@/components/icons/IconTrash.vue';

const agStore = useAccessGrantsStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const props = defineProps<{
    accessNames: string[];
}>();

const emit = defineEmits<{
    'deleted': [];
}>();

const model = defineModel<boolean>({ required: true });

async function onDeleteClick(): Promise<void> {
    await withLoading(async () => {
        try {
            const projId = projectsStore.state.selectedProject.id;
            await Promise.all(props.accessNames.map(n => agStore.deleteAccessGrantByNameAndProjectID(n, projId)));
            notify.success(`Access Grant${props.accessNames.length > 1 ? 's' : ''} deleted successfully`);
            emit('deleted');
            model.value = false;
        } catch (error) {
            error.message = `Error deleting access grant${props.accessNames.length > 1 ? 's' : ''}. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.CONFIRM_DELETE_AG_MODAL);
        }
    });
}
</script>

// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card-item class="pa-6">
        <template #prepend>
            <v-sheet
                class="border-sm d-flex justify-center align-center"
                width="40"
                height="40"
                rounded="lg"
            >
                <component :is="Box" :size="18" />
            </v-sheet>
        </template>

        <v-card-title class="font-weight-bold">Create New Project</v-card-title>

        <template #append>
            <v-btn
                :icon="X"
                variant="text"
                size="small"
                color="default"
                :disabled="isLoading"
                @click="emit('cancel')"
            />
        </template>
    </v-card-item>

    <v-divider />

    <create-project-form
        ref="form"
        @created="project => emit('created', project)"
        @update:loading="v => { isLoading = v; emit('update:loading', v) }"
    />

    <v-divider />

    <v-card-actions class="pa-6">
        <v-row>
            <v-col>
                <v-btn
                    variant="outlined"
                    color="default"
                    block
                    :disabled="isLoading"
                    @click="emit('cancel')"
                >
                    Cancel
                </v-btn>
            </v-col>
            <v-col>
                <v-btn
                    color="primary"
                    variant="flat"
                    :loading="isLoading"
                    block
                    @click="form?.submit()"
                >
                    Create Project
                </v-btn>
            </v-col>
        </v-row>
    </v-card-actions>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { VBtn, VCardActions, VCardItem, VCardTitle, VCol, VDivider, VRow, VSheet } from 'vuetify/components';
import { Box, X } from 'lucide-vue-next';

import { Project } from '@/types/projects';

import CreateProjectForm from '@/components/CreateProjectForm.vue';

const emit = defineEmits<{
    cancel: [];
    created: [project: Project];
    'update:loading': [value: boolean];
}>();

const isLoading = ref(false);
const form = ref<InstanceType<typeof CreateProjectForm>>();

function reset(): void {
    form.value?.reset();
}

defineExpose({ reset });
</script>

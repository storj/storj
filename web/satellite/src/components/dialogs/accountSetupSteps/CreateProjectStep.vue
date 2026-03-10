// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row justify="center">
            <v-col class="text-center py-4">
                <v-sheet
                    class="border-sm d-flex justify-center align-center mx-auto mb-4"
                    width="40"
                    height="40"
                    rounded="lg"
                >
                    <component :is="Box" :size="18" />
                </v-sheet>
                <h2>Create New Project</h2>
            </v-col>
        </v-row>

        <v-row justify="center">
            <v-col cols="12" sm="7" md="4" lg="3">
                <create-project-form
                    ref="form"
                    @update:loading="v => childLoading = v"
                    @created="emit('next')"
                />

                <v-row justify="center" class="mt-6">
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            :disabled="parentLoading || childLoading"
                            @click="emit('back')"
                        >
                            Back
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary"
                            variant="flat"
                            :loading="parentLoading || childLoading"
                            :disabled="!form?.formValid"
                            block
                            @click="form?.submit()"
                        >
                            Create Project
                        </v-btn>
                    </v-col>
                </v-row>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { VBtn, VCol, VContainer, VRow, VSheet } from 'vuetify/components';
import { ref } from 'vue';
import { Box } from 'lucide-vue-next';

import CreateProjectForm from '@/components/CreateProjectForm.vue';

defineProps<{
    parentLoading: boolean;
}>();

const emit = defineEmits<{
    (event: 'next'): void,
    (event: 'back'): void,
}>();

const form = ref<InstanceType<typeof CreateProjectForm>>();
const childLoading = ref<boolean>(false);
</script>

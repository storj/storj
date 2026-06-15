// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog :model-value="shouldShow" width="700px" persistent scrollable>
        <v-card rounded="xl">
            <v-card-item class="pt-2 pr-2">
                <template #append>
                    <v-btn icon color="default" density="comfortable" variant="text" size="small" @click="dismiss">
                        <x :size="18" />
                    </v-btn>
                </template>
            </v-card-item>
            <v-card-text class="d-flex justify-center">
                <img :src="projectImg" alt="Project" width="100">
            </v-card-text>
            <v-card-title class="text-center font-weight-bold text-headline-medium pt-4 px-10">
                Create your first project.
            </v-card-title>
            <v-card-text class="text-center text-medium-emphasis mx-10">
                Projects contain buckets, and buckets store your data. Usage and billing
                are tracked at the project level, making it easy to organize and manage
                your storage. Additional projects can be created on a paid plan.
            </v-card-text>
            <v-card-actions class="justify-center p-6 mt-3 mb-10">
                <v-btn color="primary" variant="flat" class="px-6" @click="openCreateProject">
                    Create Project +
                </v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>

    <create-project-dialog v-model="isCreateProjectDialogShown" />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { VBtn, VCard, VCardActions, VCardItem, VCardText, VCardTitle, VDialog } from 'vuetify/components';
import { X } from '@lucide/vue';

import { useConfigStore } from '@/store/modules/configStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { ACCOUNT_SETUP_STEPS } from '@/types/users';

import CreateProjectDialog from '@/components/dialogs/CreateProjectDialog.vue';

const projectImg = '/static/static/images/project.webp';

const configStore = useConfigStore();
const projectsStore = useProjectsStore();
const usersStore = useUsersStore();

const isCreateProjectDialogShown = ref(false);
const isDismissed = ref(false);

const shouldShow = computed<boolean>(() => {
    if (isDismissed.value) return false;
    if (!configStore.state.config.newProjectTierLockEnabled) return false;

    const settings = usersStore.state.settings;
    // Account setup is complete when onboardingEnd is true, or when the step
    // has advanced past the account setup steps (e.g. to EncryptionPassphrase).
    const accountSetupDone = settings.onboardingEnd ||
        (!!settings.onboardingStep && !ACCOUNT_SETUP_STEPS.some(s => s === settings.onboardingStep));
    if (!accountSetupDone) return false;

    return projectsStore.state.projects.length === 0;
});

function openCreateProject(): void {
    isCreateProjectDialogShown.value = true;
}

function dismiss(): void {
    isDismissed.value = true;
}
</script>

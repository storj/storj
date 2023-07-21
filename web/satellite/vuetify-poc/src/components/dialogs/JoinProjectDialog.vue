// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        min-width="400px"
        transition="fade-transition"
    >
        <v-card rounded="xlg">
            <v-card-item class="pl-7 pa-4">
                <template #prepend>
                    <v-card-title class="font-weight-bold">Join Project</v-card-title>
                </template>

                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isAccepting || isDeclining"
                        @click="model = false"
                    />
                </template>
            </v-card-item>
            <v-divider />
            <div class="px-7 py-4">Join the {{ name }} project.</div>
            <v-divider />
            <v-card-actions class="pa-7">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            :disabled="isAccepting"
                            :loading="isDeclining"
                            @click="respondToInvitation(ProjectInvitationResponse.Decline)"
                        >
                            Decline
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary"
                            variant="flat"
                            block
                            :disabled="isDeclining"
                            :loading="isAccepting"
                            @click="respondToInvitation(ProjectInvitationResponse.Accept)"
                        >
                            Join Project
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';
import {
    VDialog,
    VCard,
    VCardItem,
    VCardTitle,
    VDivider,
    VCardActions,
    VRow,
    VCol,
    VBtn,
} from 'vuetify/components';

import { ProjectInvitationResponse } from '@/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { LocalData } from '@/utils/localData';

const props = defineProps<{
    modelValue: boolean,
    name: string,
    id: string,
}>();

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

const emit = defineEmits<{
    (event: 'update:modelValue', value: boolean): void,
}>();

const projectsStore = useProjectsStore();
const router = useRouter();

const isAccepting = ref<boolean>(false);
const isDeclining = ref<boolean>(false);

/**
 * Selects the project and navigates to the project dashboard.
 */
function openProject(): void {
    projectsStore.selectProject(props.id);
    LocalData.setSelectedProjectId(props.id);
    router.push('/dashboard');
}

/**
 * Accepts or declines the project invitation.
 */
async function respondToInvitation(response: ProjectInvitationResponse): Promise<void> {
    if (isDeclining.value || isAccepting.value) return;

    const isLoading = response === ProjectInvitationResponse.Accept ? isAccepting : isDeclining;
    isLoading.value = true;

    let success = false;
    await projectsStore.respondToInvitation(props.id, response).then(() => { success = true; }).catch(_ => {});
    await projectsStore.getUserInvitations().catch(_ => {});
    await projectsStore.getProjects().catch(_ => { success = false; });

    if (response === ProjectInvitationResponse.Accept && success) openProject();

    isLoading.value = false;
}
</script>

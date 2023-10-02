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
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useNotify } from '@/utils/hooks';

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

const analyticsStore = useAnalyticsStore();
const projectsStore = useProjectsStore();
const router = useRouter();
const notify = useNotify();

const isAccepting = ref<boolean>(false);
const isDeclining = ref<boolean>(false);

/**
 * Selects the project and navigates to the project dashboard.
 */
function openProject(): void {
    projectsStore.selectProject(props.id);
    notify.success('Invite accepted!');
    router.push(`/projects/${projectsStore.state.selectedProject.urlId}/dashboard`);
    analyticsStore.pageVisit('/projects/dashboard');
}

/**
 * Accepts or declines the project invitation.
 */
async function respondToInvitation(response: ProjectInvitationResponse): Promise<void> {
    if (isDeclining.value || isAccepting.value) return;

    const accepted = response === ProjectInvitationResponse.Accept;
    const isLoading = accepted ? isAccepting : isDeclining;
    isLoading.value = true;

    let success = false;
    try {
        await projectsStore.respondToInvitation(props.id, response);
        success = true;
        analyticsStore.eventTriggered(
            accepted ?
                AnalyticsEvent.PROJECT_INVITATION_ACCEPTED :
                AnalyticsEvent.PROJECT_INVITATION_DECLINED,
        );
    } catch (error) {
        const action = accepted ? 'accept' : 'decline';
        error.message = `Failed to ${action} project invitation. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.JOIN_PROJECT_MODAL);
    }

    try {
        await projectsStore.getUserInvitations();
        await projectsStore.getProjects();
    } catch (error) {
        success = false;
        error.message = `Failed to reload projects and invitations list. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.JOIN_PROJECT_MODAL);
    }

    if (accepted && success) openProject();

    isLoading.value = false;
}
</script>

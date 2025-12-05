// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="420px"
        transition="fade-transition"
    >
        <v-card>
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-card-title class="font-weight-bold">Join Project</v-card-title>
                </template>

                <template #append>
                    <v-btn
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isAccepting || isDeclining"
                        @click="model = false"
                    />
                </template>
            </v-card-item>
            <v-divider />
            <div class="px-6 py-4">Join the {{ name }} project.</div>
            <v-divider />
            <v-card-actions class="pa-6">
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
                            Join
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue';
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
import { X } from 'lucide-vue-next';

import { ProjectInvitationResponse } from '@/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useNotify } from '@/composables/useNotify';
import { ROUTES } from '@/router';

const props = defineProps<{
    name: string,
    id: string,
}>();

const model = defineModel<boolean>({ required: true });

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
    router.push({
        name: ROUTES.Dashboard.name,
        params: { id: projectsStore.state.selectedProject.urlId },
    });
    analyticsStore.eventTriggered(AnalyticsEvent.NAVIGATE_PROJECTS);
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
            { project_id: props.id },
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

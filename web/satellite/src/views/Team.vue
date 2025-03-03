// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <trial-expiration-banner v-if="isTrialExpirationBanner && isUserProjectOwner" :expired="isExpired" />

        <PageTitleComponent title="Team Members" />
        <PageSubtitleComponent
            subtitle="Invite people and manage the team of this project."
            link="https://docs.storj.io/support/users"
        />

        <v-col>
            <v-row class="mt-1 mb-2">
                <div class="d-inline">
                    <v-btn
                        :loading="isLoading"
                        :disabled="!isLoading && !isUserAdmin"
                        :prepend-icon="UserPlus"
                        @click="onAddMember"
                    >
                        Add Members
                    </v-btn>
                    <v-tooltip v-if="!isLoading && !isUserAdmin" activator="parent" location="right">
                        Only project Owner or Admin can add new project members
                    </v-tooltip>
                </div>
            </v-row>
        </v-col>

        <TeamTableComponent :is-user-admin="isUserAdmin" />
    </v-container>

    <add-team-member-dialog v-model="isAddMemberDialogShown" :project-id="selectedProject.id" />
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { VBtn, VCol, VContainer, VRow, VTooltip } from 'vuetify/components';
import { UserPlus } from 'lucide-vue-next';

import { useProjectsStore } from '@/store/modules/projectsStore';
import { usePreCheck } from '@/composables/usePreCheck';
import { useLoading } from '@/composables/useLoading';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useNotify } from '@/composables/useNotify';
import { Project } from '@/types/projects';
import { User } from '@/types/users';
import { ProjectRole } from '@/types/projectMembers';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import TeamTableComponent from '@/components/TeamTableComponent.vue';
import AddTeamMemberDialog from '@/components/dialogs/AddTeamMemberDialog.vue';
import TrialExpirationBanner from '@/components/TrialExpirationBanner.vue';

const usersStore = useUsersStore();
const pmStore = useProjectMembersStore();
const projectsStore = useProjectsStore();

const { isTrialExpirationBanner, isUserProjectOwner, isExpired, withTrialCheck } = usePreCheck();
const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const isAddMemberDialogShown = ref<boolean>(false);
const isUserAdmin = ref<boolean>(false);

const selectedProject = computed<Project>(() => projectsStore.state.selectedProject);
const user = computed<User>(() => usersStore.state.user);

/**
 * Starts create bucket flow if user's free trial is not expired.
 */
function onAddMember(): void {
    if (isLoading.value || !isUserAdmin.value) return;

    withTrialCheck(() => {
        isAddMemberDialogShown.value = true;
    });
}

onMounted(() => {
    if (selectedProject.value.ownerId === user.value.id) {
        isUserAdmin.value = true;
        return;
    }

    withLoading(async () => {
        try {
            const pm = await pmStore.getSingleMember(selectedProject.value.id, user.value.id);
            isUserAdmin.value = pm.role === ProjectRole.Admin;
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_MEMBERS_PAGE);
        }
    });
});
</script>

// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        max-width="450px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card rounded="xlg">
            <v-sheet>
                <v-card-item class="py-4 pl-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <component :is="UserCog" :size="18" />
                        </v-sheet>
                    </template>
                    <v-card-title class="font-weight-bold">
                        Change Role
                    </v-card-title>
                    <template #append>
                        <v-btn
                            :icon="X"
                            variant="text"
                            size="small"
                            color="default"
                            @click="model = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-row>
                <v-col class="pa-6 mx-3">
                    <p class="my-2">
                        Selected team member:
                    </p>
                    <v-chip class="font-weight-bold text-wrap py-2">{{ email }}</v-chip>
                    <v-select
                        v-model="selectedRole"
                        chips
                        label="Role"
                        :items="[ProjectRole.Member, ProjectRole.Admin]"
                        class="mt-8"
                    />
                    <v-alert color="info" border variant="tonal" class="mb-4">
                        {{ selectedRole === ProjectRole.Member ?
                            'Members can only delete API keys and buckets they personally created; they cannot invite new users or remove existing users from the project.' :
                            'Admins can invite new users, remove existing users (except the project owner), and delete any API keys and buckets, regardless of who created them.' }}
                    </v-alert>
                </v-col>
            </v-row>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block @click="model = false">Cancel</v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="primary" variant="flat" block :loading="isLoading" @click="updateRole">
                            Change Role
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import {
    VAlert,
    VBtn,
    VCard,
    VCardActions,
    VCardTitle,
    VCardItem,
    VChip,
    VCol,
    VDialog,
    VDivider,
    VRow,
    VSelect,
    VSheet,
} from 'vuetify/components';
import { UserCog, X } from 'lucide-vue-next';

import { ProjectRole } from '@/types/projectMembers';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

const props = withDefaults(defineProps<{
    memberId?: string
    email?: string
}>(), {
    memberId: '',
    email: '',
});

const pmStore = useProjectMembersStore();
const projectsStore = useProjectsStore();

const model = defineModel<boolean>({ required: true });

const selectedRole = ref<ProjectRole>(ProjectRole.Member);

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

/**
 * Resends project invitation to current project.
 */
async function updateRole(): Promise<void> {
    await withLoading(async () => {
        try {
            await pmStore.updateRole(projectsStore.state.selectedProject.id, props.memberId, selectedRole.value);
            notify.success('Member role was updated successfully');
            model.value = false;
        } catch (error) {
            error.message = `Error updating role. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_MEMBERS_PAGE);
        }
    });
}
</script>

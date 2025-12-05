// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" width="auto" transition="fade-transition">
        <v-card
            rounded="xlg"
            :title="markPendingDeletion ? 'Mark Pending Deletion': 'Delete Project'"
            :subtitle="`Enter a reason for ${ markPendingDeletion ? 'marking' : 'deleting'} this project ${ markPendingDeletion ? 'for deletion' : '' }`"
        >
            <template #append>
                <v-btn
                    :icon="X" :disabled="isLoading"
                    variant="text" size="small" color="default" @click="model = false"
                />
            </template>

            <div class="pa-6">
                <v-row>
                    <v-col cols="12">
                        <v-text-field
                            :model-value="project.id"
                            label="Project ID"
                            variant="solo-filled"
                            hide-details="auto"
                            flat readonly
                        />
                    </v-col>
                    <v-col cols="12">
                        <v-text-field
                            :model-value="project.owner.email"
                            label="Owner Email"
                            variant="solo-filled"
                            hide-details="auto"
                            flat readonly
                        />
                    </v-col>
                    <v-col cols="12">
                        <v-textarea
                            v-model="reason"
                            :rules="[RequiredRule]"
                            label="Reason"
                            :placeholder="`Enter a reason for ${ markPendingDeletion ? 'marking' : 'deleting'} this project ${ markPendingDeletion ? 'for deletion' : '' }`"
                            variant="solo-filled"
                            hide-details="auto"
                            autofocus
                            flat
                        />
                    </v-col>
                </v-row>

                <v-alert class="mt-6" title="Warning" variant="tonal" color="error" rounded="lg">
                    <template v-if="markPendingDeletion">
                        This will set status to "<strong>Pending Deletion</strong>".
                        <br>
                        The project will be deleted later by a chore.
                    </template>
                    <template v-else>
                        This will delete the project and data.
                    </template>
                </v-alert>
            </div>

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block @click="model = false">Cancel</v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="error" variant="flat"
                            :loading="isLoading"
                            :disabled="!reason"
                            block
                            @click="disableProject"
                        >
                            {{ markPendingDeletion ? 'Mark Pending Deletion' : 'Delete Project' }}
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { VAlert, VBtn, VCard, VCardActions, VCol, VDialog, VRow, VTextarea, VTextField } from 'vuetify/components';
import { X } from 'lucide-vue-next';
import { ref, watch } from 'vue';

import { useLoading } from '@/composables/useLoading';
import { Project } from '@/api/client.gen';
import { useNotify } from '@/composables/useNotify';
import { RequiredRule } from '@/types/common';
import { useProjectsStore } from '@/store/projects';
import { useUsersStore } from '@/store/users';
import { ProjectStatus } from '@/types/project';

const notify = useNotify();
const projectsStore = useProjectsStore();
const usersStore = useUsersStore();
const { isLoading, withLoading } = useLoading();

const model = defineModel<boolean>({ required: true });
const markPendingDeletion = defineModel<boolean>('markPendingDeletion', { default: false });

const props = defineProps<{
    project: Project;
}>();

const reason = ref('');

function disableProject() {
    withLoading(async () => {
        try {
            await projectsStore.disableProject(props.project.id, markPendingDeletion.value, reason.value);
            notify.success(`Project ${markPendingDeletion.value ? 'marked for deletion' : 'deleted'} successfully`);

            const currentUser = usersStore.state.currentAccount;
            if (!currentUser) return;
            const user = { ...currentUser };
            if (!user.projects) return;

            const index = user.projects?.findIndex(p => p.id === props.project.id) ?? -1;
            if (index === -1) return;

            user.projects[index].active = false;
            await usersStore.updateCurrentUser(user);
            model.value = false;

            if (projectsStore.state.currentProject?.id !== props.project.id) return;
            const project = { ...props.project };
            if (project.status === undefined || project.status === null) return;

            project.status.value = markPendingDeletion.value ?
                ProjectStatus.PendingDeletion :
                ProjectStatus.Disabled;
            await projectsStore.updateCurrentProject(project);
        } catch (e) {
            notify.error(e);
        }
    });
}

watch(model, (newVal) => {
    if (!newVal && markPendingDeletion.value) markPendingDeletion.value = false;
    if (newVal) reason.value = '';
});
</script>

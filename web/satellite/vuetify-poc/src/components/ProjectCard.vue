// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="flat" :border="true" rounded="xlg">
        <div class="h-100 d-flex flex-column justify-space-between">
            <v-card-item>
                <div class="d-flex justify-space-between">
                    <v-chip rounded :color="item ? PROJECT_ROLE_COLORS[item.role] : 'primary'" variant="tonal" class="font-weight-bold my-2" size="small">
                        <icon-project width="12px" class="mr-1" />
                        {{ item?.role || 'Project' }}
                    </v-chip>

                    <v-btn v-if="item?.role === ProjectRole.Owner" color="default" variant="text" size="small">
                        <v-icon icon="mdi-dots-vertical" />

                        <v-menu activator="parent" location="end" transition="scale-transition">
                            <v-list class="pa-2">
                                <v-list-item link rounded="lg">
                                    <template #prepend>
                                        <icon-settings />
                                    </template>
                                    <v-list-item-title class="text-body-2 ml-3">
                                        Project Settings
                                    </v-list-item-title>
                                </v-list-item>

                                <v-divider class="my-2" />

                                <v-list-item link class="mt-1" rounded="lg">
                                    <template #prepend>
                                        <icon-team />
                                    </template>
                                    <v-list-item-title class="text-body-2 ml-3">
                                        Invite Members
                                    </v-list-item-title>
                                </v-list-item>
                            </v-list>
                        </v-menu>
                    </v-btn>
                </div>
                <v-card-title :class="{ 'text-primary': item && item.role !== ProjectRole.Invited }">
                    <a v-if="item && item.role !== ProjectRole.Invited" class="link" @click="openProject">
                        {{ item.name }}
                    </a>
                    <template v-else>
                        {{ item ? item.name : 'Welcome' }}
                    </template>
                </v-card-title>
                <v-card-subtitle v-if="!item || item.description">
                    {{ item ? item.description : 'Create a project to get started.' }}
                </v-card-subtitle>
            </v-card-item>
            <v-card-text class="flex-grow-0">
                <v-divider class="mt-1 mb-4" />
                <v-btn v-if="!item" color="primary" size="small" class="mr-2">Create Project</v-btn>
                <template v-else-if="item?.role === ProjectRole.Invited">
                    <v-btn color="primary" size="small" class="mr-2" :disabled="isDeclining" @click="emit('joinClick')">
                        Join Project
                    </v-btn>
                    <v-btn
                        variant="outlined"
                        color="default"
                        size="small"
                        class="mr-2"
                        :loading="isDeclining"
                        @click="declineInvitation"
                    >
                        Decline
                    </v-btn>
                </template>
                <v-btn v-else color="primary" size="small" class="mr-2" @click="openProject">Open Project</v-btn>
            </v-card-text>
        </div>
    </v-card>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import {
    VCard,
    VCardItem,
    VChip,
    VBtn,
    VIcon,
    VMenu,
    VList,
    VListItem,
    VListItemTitle,
    VDivider,
    VCardTitle,
    VCardSubtitle,
    VCardText,
} from 'vuetify/components';

import { ProjectItemModel, PROJECT_ROLE_COLORS } from '@poc/types/projects';
import { ProjectInvitationResponse } from '@/types/projects';
import { ProjectRole } from '@/types/projectMembers';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { LocalData } from '@/utils/localData';

import IconProject from '@poc/components/icons/IconProject.vue';
import IconSettings from '@poc/components/icons/IconSettings.vue';
import IconTeam from '@poc/components/icons/IconTeam.vue';

const props = defineProps<{
    item?: ProjectItemModel,
}>();

const emit = defineEmits<{
    (event: 'joinClick'): void;
}>();

const projectsStore = useProjectsStore();
const router = useRouter();

const isDeclining = ref<boolean>(false);

/**
 * Selects the project and navigates to the project dashboard.
 */
function openProject(): void {
    if (!props.item) return;
    projectsStore.selectProject(props.item.id);
    LocalData.setSelectedProjectId(props.item.id);
    router.push('/dashboard');
}

/**
 * Declines the project invitation.
 */
async function declineInvitation(): Promise<void> {
    if (!props.item || isDeclining.value) return;
    isDeclining.value = true;

    await projectsStore.respondToInvitation(props.item.id, ProjectInvitationResponse.Decline).catch(_ => {});
    await projectsStore.getUserInvitations().catch(_ => {});
    await projectsStore.getProjects().catch(_ => {});

    isDeclining.value = false;
}
</script>

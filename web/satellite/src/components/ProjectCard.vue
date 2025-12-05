// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card class="pa-2 h-100">
        <div class="h-100 d-flex flex-column justify-space-between">
            <v-card-item>
                <div class="d-flex justify-space-between">
                    <v-chip :color="item ? PROJECT_ROLE_COLORS[item.role] : 'primary'" variant="tonal" class="font-weight-bold mt-1 mb-2" size="small">
                        <component :is="Box" :size="12" class="mr-1" />
                        {{ item?.role || 'Project' }}
                    </v-chip>
                    <v-chip v-if="item && item.isClassic" variant="tonal" color="warning" size="small" class="font-weight-bold">
                        Classic
                        <v-tooltip activator="parent" location="top">
                            Pricing from before Nov 2025.
                            <a class="link" @click="isMigrateDialog = true">Migrate Now</a>
                        </v-tooltip>
                    </v-chip>
                </div>
                <v-card-title :class="{ 'text-primary': item && item.role !== ProjectRole.Invited }">
                    <a v-if="item && item.role !== ProjectRole.Invited" class="link text-decoration-none" @click="openProject">
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
                <v-btn v-if="!item" color="primary" class="mr-2" @click="emit('createClick')">
                    Create Project
                </v-btn>
                <template v-else-if="item?.role === ProjectRole.Invited">
                    <v-btn color="primary" class="mr-2" :disabled="isDeclining" @click="emit('joinClick')">
                        Join Project
                    </v-btn>
                    <v-btn
                        variant="outlined"
                        color="default"
                        class="mr-2"
                        :loading="isDeclining"
                        @click="declineInvitation"
                    >
                        Decline
                    </v-btn>
                </template>
                <v-btn v-else color="primary" class="mr-2" @click="openProject">Open Project</v-btn>

                <v-menu v-if="item?.role === ProjectRole.Owner" location="bottom" transition="fade-transition">
                    <template #activator="{ props: menuProps }">
                        <v-btn v-bind="menuProps" color="default" variant="outlined" density="comfortable" icon>
                            <v-icon :icon="Ellipsis" />
                        </v-btn>
                    </template>

                    <v-list class="pa-1">
                        <v-list-item link @click="emit('inviteClick')">
                            <template #prepend>
                                <component :is="UserPlus" :size="18" />
                            </template>
                            <v-list-item-title class="text-body-2 ml-3">
                                Add Members
                            </v-list-item-title>
                        </v-list-item>

                        <v-divider />

                        <v-list-item link @click="() => editClick(FieldToChange.Name)">
                            <template #prepend>
                                <component :is="Pencil" :size="18" />
                            </template>
                            <v-list-item-title class="text-body-2 ml-3">
                                Edit Name
                            </v-list-item-title>
                        </v-list-item>

                        <v-list-item link @click="() => editClick(FieldToChange.Description)">
                            <template #prepend>
                                <component :is="NotebookPen" :size="18" />
                            </template>
                            <v-list-item-title class="text-body-2 ml-3">
                                Edit Description
                            </v-list-item-title>
                        </v-list-item>

                        <v-divider />

                        <v-list-item v-if="isPaidTier" link @click="() => updateLimitsClick(LimitToChange.Storage)">
                            <template #prepend>
                                <component :is="Cloud" :size="18" />
                            </template>
                            <v-list-item-title class="text-body-2 ml-3">
                                Edit Storage Limit
                            </v-list-item-title>
                        </v-list-item>

                        <v-list-item v-if="isPaidTier" link @click="() => updateLimitsClick(LimitToChange.Bandwidth)">
                            <template #prepend>
                                <component :is="DownloadCloud" :size="18" />
                            </template>
                            <v-list-item-title class="text-body-2 ml-3">
                                Edit Download Limit
                            </v-list-item-title>
                        </v-list-item>

                        <v-divider />

                        <v-list-item link @click="() => onSettingsClick()">
                            <template #prepend>
                                <component :is="Settings" :size="18" />
                            </template>
                            <v-list-item-title class="text-body-2 ml-3">
                                Project Settings
                            </v-list-item-title>
                        </v-list-item>

                        <v-list-item v-if="item && item.isClassic" link @click="isMigrateDialog = true">
                            <template #prepend>
                                <component :is="CircleFadingArrowUp" :size="18" />
                            </template>
                            <v-list-item-title class="text-body-2 ml-3">
                                Migrate Project
                            </v-list-item-title>
                        </v-list-item>
                    </v-list>
                </v-menu>
            </v-card-text>
        </div>
    </v-card>

    <migrate-project-pricing-dialog v-if="props.item" v-model="isMigrateDialog" :project-id="props.item.id" />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';
import {
    VBtn,
    VCard,
    VCardItem,
    VCardSubtitle,
    VCardText,
    VCardTitle,
    VChip,
    VIcon,
    VList,
    VListItem,
    VListItemTitle,
    VMenu,
    VDivider,
    VTooltip,
} from 'vuetify/components';
import {
    Box,
    Cloud,
    DownloadCloud,
    Ellipsis,
    Pencil,
    NotebookPen,
    Settings,
    UserPlus,
    CircleFadingArrowUp,
} from 'lucide-vue-next';

import {
    FieldToChange,
    LimitToChange,
    PROJECT_ROLE_COLORS,
    ProjectInvitationResponse,
    ProjectItemModel,
} from '@/types/projects';
import { ProjectRole } from '@/types/projectMembers';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { ROUTES } from '@/router';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useUsersStore } from '@/store/modules/usersStore';

import MigrateProjectPricingDialog from '@/components/dialogs/MigrateProjectPricingDialog.vue';

const props = defineProps<{
    item?: ProjectItemModel,
}>();

const emit = defineEmits<{
    joinClick: [];
    createClick: [];
    inviteClick: [];
    editClick: [FieldToChange];
    updateLimitsClick: [LimitToChange];
}>();

const analyticsStore = useAnalyticsStore();
const bucketsStore = useBucketsStore();
const projectsStore = useProjectsStore();
const userStore = useUsersStore();
const router = useRouter();
const notify = useNotify();

const isDeclining = ref<boolean>(false);
const isMigrateDialog = ref<boolean>(false);

const isPaidTier = computed(() => userStore.state.user.isPaid);

/**
 * Selects the project and navigates to the project dashboard.
 */
function openProject(): void {
    if (!props.item) return;

    // There is no reason to clear s3 data if the user is navigating to the previously selected project.
    if (projectsStore.state.selectedProject.id !== props.item.id) bucketsStore.clearS3Data();

    projectsStore.selectProject(props.item.id);

    router.push({
        name: ROUTES.Dashboard.name,
        params: { id: projectsStore.state.selectedProject.urlId },
    });
    analyticsStore.eventTriggered(AnalyticsEvent.NAVIGATE_PROJECTS);
}

/**
 * Selects the project and navigates to the project's settings.
 */
function onSettingsClick(): void {
    if (!props.item) return;
    projectsStore.selectProject(props.item.id);
    router.push({
        name: ROUTES.ProjectSettings.name,
        params: { id: projectsStore.state.selectedProject.urlId },
    });
}

function editClick(field: FieldToChange): void {
    if (!props.item) return;
    emit('editClick', field);
}

function updateLimitsClick(limit: LimitToChange): void {
    if (!props.item) return;
    emit('updateLimitsClick', limit);
}

/**
 * Declines the project invitation.
 */
async function declineInvitation(): Promise<void> {
    if (!props.item || isDeclining.value) return;
    isDeclining.value = true;

    try {
        await projectsStore.respondToInvitation(props.item.id, ProjectInvitationResponse.Decline);
        analyticsStore.eventTriggered(AnalyticsEvent.PROJECT_INVITATION_DECLINED, { project_id: props.item.id });
    } catch (error) {
        error.message = `Failed to decline project invitation. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_INVITATION);
    }

    try {
        await projectsStore.getUserInvitations();
        await projectsStore.getProjects();
    } catch (error) {
        error.message = `Failed to reload projects and invitations list. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_INVITATION);
    }

    isDeclining.value = false;
}
</script>

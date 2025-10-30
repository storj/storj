// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-text-field
        v-model="search"
        label="Search"
        :prepend-inner-icon="Search"
        single-line
        variant="solo-filled"
        flat
        hide-details
        clearable
        density="comfortable"
        rounded="lg"
        class="mb-4"
    />

    <v-data-table
        :sort-by="sortBy"
        :headers="headers"
        :items="items"
        items-per-page="10"
        items-per-page-text="Projects per page"
        :search="search"
        no-data-text="No results found"
        item-key="path"
    >
        <template #item.name="{ item }">
            <v-btn
                v-if="item.role !== ProjectRole.Invited"
                class="rounded-lg pl-1 pr-4 ml-n1 justify-start font-weight-bold"
                variant="text"
                height="40"
                color="default"
                block
                @click="openProject(item)"
            >
                <img src="@/assets/icon-project-tonal.svg" alt="Project" class="mr-3">
                {{ item.name }}
                <v-chip v-if="item.isClassic" variant="tonal" color="warning" size="small" class="font-weight-bold ml-2">
                    Classic
                    <v-tooltip activator="parent" location="top">
                        Pricing from before Nov 2025.
                        <a class="link" @click="() => onMigrateClick(item.id)">Migrate Now</a>
                    </v-tooltip>
                </v-chip>
            </v-btn>
            <div v-else class="pl-1 pr-4 ml-n1 d-flex align-center justify-start font-weight-bold">
                <img src="@/assets/icon-project-tonal.svg" alt="Project" class="mr-3">
                <span class="text-no-wrap">{{ item.name }}</span>
            </div>
        </template>

        <template #item.role="{ item }">
            <v-chip :color="PROJECT_ROLE_COLORS[item.role]" size="small" class="font-weight-bold">
                {{ item.role }}
            </v-chip>
        </template>

        <template #item.storageUsed="{ item }">
            <span class="text-no-wrap">
                {{ formattedValue(new Size(item.storageUsed, 2)) }}
            </span>
        </template>

        <template #item.bandwidthUsed="{ item }">
            <span class="text-no-wrap">
                {{ formattedValue(new Size(item.bandwidthUsed, 2)) }}
            </span>
        </template>

        <template #item.encryption="{ item }">
            <v-chip v-if="item.encryption" color="primary" variant="tonal" class="font-weight-bold" size="small">
                {{ item.encryption }}
            </v-chip>
        </template>

        <template #item.createdAt="{ item }">
            <span class="text-no-wrap">
                {{ Time.formattedDate(item.createdAt) }}
            </span>
        </template>

        <template #item.actions="{ item }">
            <div class="w-100 d-flex align-center justify-space-between">
                <v-btn
                    v-if="item.role === ProjectRole.Invited"
                    color="primary"
                    size="small"
                    :disabled="decliningIds.has(item.id)"
                    @click="emit('joinClick', item)"
                >
                    Join Project
                </v-btn>
                <v-btn v-else color="primary" size="small" rounded="md" @click="openProject(item)">Open Project</v-btn>

                <v-btn
                    v-if="item.role === ProjectRole.Owner || item.role === ProjectRole.Invited"
                    class="ml-2"
                    icon
                    color="default"
                    variant="outlined"
                    size="small"
                    rounded="md"
                    density="comfortable"
                    :loading="decliningIds.has(item.id)"
                >
                    <v-icon :icon="Ellipsis" size="18" />
                    <v-menu activator="parent" location="bottom" transition="scale-transition">
                        <v-list class="pa-1">
                            <template v-if="item.role === ProjectRole.Owner">
                                <v-list-item link @click="emit('inviteClick', item)">
                                    <template #prepend>
                                        <component :is="UserPlus" :size="18" />
                                    </template>
                                    <v-list-item-title class="text-body-2 ml-3">
                                        Add Members
                                    </v-list-item-title>
                                </v-list-item>

                                <v-divider />

                                <v-list-item link @click="() => emit('editClick', item, FieldToChange.Name)">
                                    <template #prepend>
                                        <component :is="Pencil" :size="18" />
                                    </template>
                                    <v-list-item-title class="text-body-2 ml-3">
                                        Edit Name
                                    </v-list-item-title>
                                </v-list-item>

                                <v-list-item link @click="() => emit('editClick', item, FieldToChange.Description)">
                                    <template #prepend>
                                        <component :is="NotebookPen" :size="18" />
                                    </template>
                                    <v-list-item-title class="text-body-2 ml-3">
                                        Edit Description
                                    </v-list-item-title>
                                </v-list-item>

                                <v-divider />

                                <v-list-item v-if="hasPaidPrivileges" link @click="() => emit('updateLimitsClick', item, LimitToChange.Storage)">
                                    <template #prepend>
                                        <component :is="Cloud" :size="18" />
                                    </template>
                                    <v-list-item-title class="text-body-2 ml-3">
                                        Edit Storage Limit
                                    </v-list-item-title>
                                </v-list-item>

                                <v-list-item v-if="hasPaidPrivileges" link @click="() => emit('updateLimitsClick', item, LimitToChange.Bandwidth)">
                                    <template #prepend>
                                        <component :is="DownloadCloud" :size="18" />
                                    </template>
                                    <v-list-item-title class="text-body-2 ml-3">
                                        Edit Download Limit
                                    </v-list-item-title>
                                </v-list-item>

                                <v-divider />

                                <v-list-item link @click="() => onSettingsClick(item)">
                                    <template #prepend>
                                        <component :is="Settings" :size="18" />
                                    </template>
                                    <v-list-item-title class="text-body-2 ml-3">
                                        Project Settings
                                    </v-list-item-title>
                                </v-list-item>

                                <v-list-item v-if="item.isClassic" link @click="() => onMigrateClick(item.id)">
                                    <template #prepend>
                                        <component :is="CircleFadingArrowUp" :size="18" />
                                    </template>
                                    <v-list-item-title class="text-body-2 ml-3">
                                        Migrate Project
                                    </v-list-item-title>
                                </v-list-item>
                            </template>
                            <v-list-item v-else link @click="declineInvitation(item)">
                                <template #prepend>
                                    <component :is="Trash2" :size="18" />
                                </template>
                                <v-list-item-title class="text-body-2 ml-3">
                                    Decline
                                </v-list-item-title>
                            </v-list-item>
                        </v-list>
                    </v-menu>
                </v-btn>
            </div>
        </template>
    </v-data-table>

    <migrate-project-pricing-dialog v-model="isMigrateDialog" :project-id="projectIDToMigrate" />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';
import {
    VTextField,
    VListItem,
    VChip,
    VBtn,
    VMenu,
    VList,
    VIcon,
    VListItemTitle,
    VDataTable,
    VDivider,
    VTooltip,
} from 'vuetify/components';
import {
    Ellipsis,
    Search,
    UserPlus,
    Settings,
    Trash2,
    Cloud,
    DownloadCloud,
    Pencil,
    NotebookPen,
    CircleFadingArrowUp,
} from 'lucide-vue-next';

import { Time } from '@/utils/time';
import {
    ProjectItemModel,
    PROJECT_ROLE_COLORS,
    ProjectInvitationResponse,
    FieldToChange,
    LimitToChange,
} from '@/types/projects';
import { ProjectRole } from '@/types/projectMembers';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { ROUTES } from '@/router';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { DataTableHeader, SortItem } from '@/types/common';
import { useUsersStore } from '@/store/modules/usersStore';
import { Dimensions, Size } from '@/utils/bytesSize';
import { useConfigStore } from '@/store/modules/configStore';

import MigrateProjectPricingDialog from '@/components/dialogs/MigrateProjectPricingDialog.vue';

defineProps<{
    items: ProjectItemModel[],
}>();

const emit = defineEmits<{
    (event: 'joinClick', item: ProjectItemModel): void;
    (event: 'inviteClick', item: ProjectItemModel): void;
    (event: 'editClick', item: ProjectItemModel, field: FieldToChange): void;
    (event: 'updateLimitsClick', item: ProjectItemModel, limit: LimitToChange): void;
}>();

const search = ref<string>('');
const decliningIds = ref(new Set<string>());
const projectIDToMigrate = ref<string>('');
const isMigrateDialog = ref<boolean>(false);

const analyticsStore = useAnalyticsStore();
const bucketsStore = useBucketsStore();
const projectsStore = useProjectsStore();
const userStore = useUsersStore();
const configStore = useConfigStore();

const router = useRouter();
const notify = useNotify();

const sortBy: SortItem[] = [{ key: 'name', order: 'asc' }];

const hasPaidPrivileges = computed(() => userStore.state.user.hasPaidPrivileges);

const satelliteManagedEncryptionEnabled = computed<boolean>(() => configStore.state.config.satelliteManagedEncryptionEnabled);

const headers = computed<DataTableHeader[]>(() => {
    const hdrs: DataTableHeader[] = [
        { title: 'Project', key: 'name', align: 'start' },
        { title: 'Role', key: 'role' },
        { title: 'Members', key: 'memberCount' },
        { title: 'Storage', key: 'storageUsed' },
        { title: 'Download', key: 'bandwidthUsed' },
    ];
    if (satelliteManagedEncryptionEnabled.value) {
        hdrs.push({ title: 'Encryption', key: 'encryption', sortable: false });
    }
    hdrs.push(
        { title: 'Date Added', key: 'createdAt' },
        { title: '', key: 'actions', sortable: false, width: '0' },
    );

    return hdrs;
});

/**
 * Formats value to needed form and returns it.
 */
function formattedValue(value: Size): string {
    switch (value.label) {
    case Dimensions.Bytes:
        return '0';
    default:
        return `${value.formattedBytes.replace(/\.0+$/, '')}${value.label}`;
    }
}

/**
 * Selects the project and navigates to the project dashboard.
 */
function openProject(item: ProjectItemModel): void {
    // There is no reason to clear s3 data if the user is navigating to the previously selected project.
    if (projectsStore.state.selectedProject.id !== item.id) bucketsStore.clearS3Data();

    projectsStore.selectProject(item.id);

    router.push({
        name: ROUTES.Dashboard.name,
        params: { id: projectsStore.state.selectedProject.urlId },
    });
    analyticsStore.eventTriggered(AnalyticsEvent.NAVIGATE_PROJECTS);
}

/**
 * Selects the project and navigates to the project's settings.
 */
function onSettingsClick(item: ProjectItemModel): void {
    projectsStore.selectProject(item.id);
    router.push({
        name: ROUTES.ProjectSettings.name,
        params: { id: projectsStore.state.selectedProject.urlId },
    });
}

function onMigrateClick(id: string): void {
    projectIDToMigrate.value = id;
    isMigrateDialog.value = true;
}

/**
 * Declines the project invitation.
 */
async function declineInvitation(item: ProjectItemModel): Promise<void> {
    if (decliningIds.value.has(item.id)) return;
    decliningIds.value.add(item.id);

    try {
        await projectsStore.respondToInvitation(item.id, ProjectInvitationResponse.Decline);
        analyticsStore.eventTriggered(AnalyticsEvent.PROJECT_INVITATION_DECLINED, { project_id: item.id });
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

    decliningIds.value.delete(item.id);
}
</script>

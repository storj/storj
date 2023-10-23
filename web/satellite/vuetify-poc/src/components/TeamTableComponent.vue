// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card min-height="24" :border="0" class="mb-2 elevation-0">
        <v-row v-if="selectedMembers.length > 0" class="justify-end align-center ma-0">
            <p>{{ selectedMembers.length }} user{{ selectedMembers.length !== 1 ? 's' : '' }} selected</p>
        </v-row>
    </v-card>
    <v-card variant="flat" :border="true" rounded="xlg">
        <v-row align="center" class="ma-0">
            <v-col v-if="selectedMembers.length" class="px-0 mr-0 ml-2 mt-2" cols="auto">
                <v-btn
                    class=" text-caption"
                    prepend-icon="mdi-trash-can-outline"
                    variant="outlined"
                    color="default"
                    @click="showDeleteDialog"
                >
                    Remove
                </v-btn>
            </v-col>

            <v-col class="px-0 mx-2 mt-2">
                <v-text-field
                    v-model="search"
                    label="Search"
                    prepend-inner-icon="mdi-magnify"
                    single-line
                    variant="solo-filled"
                    flat
                    hide-details
                    clearable
                    density="comfortable"
                    rounded="lg"
                />
            </v-col>
        </v-row>

        <v-data-table-server
            v-model="selectedMembers"
            :search="search"
            :headers="headers"
            :items="projectMembers"
            :loading="isLoading"
            :items-length="page.totalCount"
            :items-per-page-options="tableSizeOptions(page.totalCount)"
            no-data-text="No results found"
            class="elevation-1"
            item-value="email"
            select-strategy="all"
            item-selectable="selectable"
            show-select
            hover
            @update:itemsPerPage="onUpdateLimit"
            @update:page="onUpdatePage"
            @update:sortBy="onUpdateSortBy"
        >
            <template #item.name="{ item }">
                <span class="font-weight-bold">
                    {{ item.raw.name }}
                </span>
            </template>
            <template #item.role="{ item }">
                <v-chip :color="PROJECT_ROLE_COLORS[item.raw.role]" variant="tonal" size="small" rounded="xl" class="font-weight-bold">
                    {{ item.raw.role }}
                </v-chip>
            </template>
            <template #item.actions="{ item }">
                <v-btn
                    v-if="item.raw.role !== ProjectRole.Owner"
                    variant="outlined"
                    color="default"
                    size="small"
                    class="mr-1 text-caption"
                    density="comfortable"
                    icon
                >
                    <v-icon icon="mdi-dots-horizontal" />
                    <v-menu activator="parent">
                        <v-list class="py-2">
                            <v-list-item
                                density="comfortable"
                                link
                                rounded="lg"
                                @click="() => onResendOrCopyClick(item.raw.expired, item.raw.email)"
                            >
                                <template #prepend>
                                    <icon-upload v-if="item.raw.expired" :size="18" />
                                    <icon-copy v-else />
                                </template>
                                <v-list-item-title class="pl-2 text-body-2 font-weight-medium">
                                    {{ item.raw.expired ? 'Resend invite' : 'Copy invite link' }}
                                </v-list-item-title>
                            </v-list-item>
                            <v-list-item
                                class="text-error"
                                density="comfortable"
                                link rounded="lg"
                                @click="() => onSingleDelete(item.raw.email)"
                            >
                                <template #prepend>
                                    <icon-trash bold />
                                </template>
                                <v-list-item-title class="pl-2 text-body-2 font-weight-medium">
                                    Remove member
                                </v-list-item-title>
                            </v-list-item>
                        </v-list>
                    </v-menu>
                </v-btn>
            </template>
        </v-data-table-server>
    </v-card>

    <remove-project-member-dialog
        v-model="isRemoveMembersDialogShown"
        :emails="selectedMembers"
        @deleted="onPostDelete"
    />
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import {
    VRow,
    VCard,
    VCol,
    VTextField,
    VChip,
    VIcon,
    VList,
    VMenu,
    VListItem,
    VBtn,
    VListItemTitle,
} from 'vuetify/components';
import { VDataTableServer } from 'vuetify/labs/components';
import { useRouter } from 'vue-router';

import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import {
    ProjectInvitationItemModel,
    ProjectMemberCursor,
    ProjectMemberOrderBy,
    ProjectMembersPage,
    ProjectRole,
} from '@/types/projectMembers';
import { PROJECT_ROLE_COLORS } from '@poc/types/projects';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useAppStore } from '@/store/modules/appStore';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { Project } from '@/types/projects';
import { SortDirection, tableSizeOptions } from '@/types/common';
import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';

import IconTrash from '@poc/components/icons/IconTrash.vue';
import IconCopy from '@poc/components/icons/IconCopy.vue';
import IconUpload from '@poc/components/icons/IconUpload.vue';
import RemoveProjectMemberDialog from '@poc/components/dialogs/RemoveProjectMemberDialog.vue';

type RenderedItem = {
    name: string,
    email: string,
    role: ProjectRole,
    date: string,
    selectable: boolean,
    expired: boolean,
}

const usersStore = useUsersStore();
const analyticsStore = useAnalyticsStore();
const pmStore = useProjectMembersStore();
const projectsStore = useProjectsStore();
const configStore = useConfigStore();

const router = useRouter();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const isRemoveMembersDialogShown = ref<boolean>(false);
const search = ref<string>('');
const searchTimer = ref<NodeJS.Timeout>();
const sortBy = ref([{ key: 'date', order: 'asc' }]);
const selectedMembers = ref<string[]>([]);

const headers = ref([
    {
        title: 'Name',
        align: 'start',
        key: 'name',
    },
    { title: 'Email', key: 'email' },
    { title: 'Role', key: 'role' },
    { title: 'Date Added', key: 'date' },
    { title: '', key: 'actions', sortable: false, width: 0 },
]);

const selectedProject = computed<Project>(() => projectsStore.state.selectedProject);
const cursor = computed<ProjectMemberCursor>(() => pmStore.state.cursor);
const page = computed<ProjectMembersPage>(() => pmStore.state.page as ProjectMembersPage);

const FIRST_PAGE = 1;
const inviteLinksCache = new Map<string, string>();

/**
 * Returns team members of current page from store.
 * With project owner pinned to top
 */
const projectMembers = computed((): RenderedItem[] => {
    const projectMembers = page.value.getAllItems();
    const projectOwner = projectMembers.find((member) => member.getUserID() === selectedProject.value.ownerId);
    const projectMembersToReturn = projectMembers.filter((member) => member.getUserID() !== selectedProject.value.ownerId);

    // if the project owner exists, place at the front of the members list
    projectOwner && projectMembersToReturn.unshift(projectOwner);

    return projectMembersToReturn.map(member => {
        let role = ProjectRole.Member;
        if (member.getUserID() === projectOwner?.getUserID()) {
            role = ProjectRole.Owner;
        } else if (member.isPending()) {
            if ((member as ProjectInvitationItemModel).expired) {
                role = ProjectRole.InviteExpired;
            } else {
                role = ProjectRole.Invited;
            }
        }

        return {
            name: member.getName(),
            email: member.getEmail(),
            role,
            date: member.getJoinDate().toLocaleDateString('en-US', { day:'numeric', month:'short', year:'numeric' }),
            selectable: role !== ProjectRole.Owner,
            expired: member.isPending() && 'expired' in member && Boolean(member.expired),
        };
    });
});

/**
 * Handles update table rows limit event.
 */
async function onUpdateLimit(limit: number): Promise<void> {
    await fetch(page.value.currentPage, limit);
}

/**
 * Handles update table page event.
 */
async function onUpdatePage(page: number): Promise<void> {
    await fetch(page, cursor.value.limit);
}

/**
 * Handles post delete operations.
 */
async function onPostDelete(): Promise<void> {
    if (selectedMembers.value.includes(usersStore.state.user.email)) {
        router.push('/projects');
        return;
    }

    search.value = '';
    selectedMembers.value = [];
    await onUpdatePage(FIRST_PAGE);
}

function onSingleDelete(email: string): void {
    selectedMembers.value = [email];
    isRemoveMembersDialogShown.value = true;
}

/**
 * Handles update table sorting event.
 */
async function onUpdateSortBy(sortBy: {key: keyof ProjectMemberOrderBy, order: keyof SortDirection}[]): Promise<void> {
    if (!sortBy.length) return;

    const sorting = sortBy[0];

    pmStore.setSortingBy(ProjectMemberOrderBy[sorting.key]);
    pmStore.setSortingDirection(SortDirection[sorting.order]);

    await fetch(FIRST_PAGE, cursor.value.limit);
}

/**
 * Handles on invite raw action click logic depending on expiration status.
 */
async function onResendOrCopyClick(expired: boolean, email: string): Promise<void> {
    expired ? await resendInvite(email) : await copyInviteLink(email);
}

/**
 * Resends project invitation to current project.
 */
async function resendInvite(email: string): Promise<void> {
    await withLoading(async () => {
        analyticsStore.eventTriggered(AnalyticsEvent.RESEND_INVITE_CLICKED);
        try {
            await pmStore.reinviteMembers([email], selectedProject.value.id);
            if (configStore.state.config.unregisteredInviteEmailsEnabled) {
                notify.notify('Invite re-sent!');
            } else {
                notify.notify(
                    'The invitation will be re-sent to the email address if it belongs to a user on this satellite.',
                    'Invite re-sent!',
                );
            }
        } catch (error) {
            error.message = `Error resending invite. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_MEMBERS_PAGE);
            return;
        }

        await onUpdatePage(FIRST_PAGE);
    });
}

/**
 * Copies project invitation link to user's clipboard.
 */
async function copyInviteLink(email: string): Promise<void> {
    analyticsStore.eventTriggered(AnalyticsEvent.COPY_INVITE_LINK_CLICKED);

    const cachedLink = inviteLinksCache.get(email);
    if (cachedLink) {
        await navigator.clipboard.writeText(cachedLink);
        notify.notify('Invite copied!');
        return;
    }
    await withLoading(async () => {
        try {
            const link = await pmStore.getInviteLink(email, selectedProject.value.id);
            await navigator.clipboard.writeText(link);
            inviteLinksCache.set(email, link);
            notify.notify('Invite copied!');
        } catch (error) {
            error.message = `Error getting invite link. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_MEMBERS_PAGE);
        }
    });
}

/**
 * Fetches Project members records depending on page and limit.
 */
async function fetch(page = FIRST_PAGE, limit = DEFAULT_PAGE_LIMIT): Promise<void> {
    await withLoading(async () => {
        try {
            await pmStore.getProjectMembers(page, selectedProject.value.id, limit);
        } catch (error) {
            notify.error(`Unable to fetch Project Members. ${error.message}`, AnalyticsErrorEventSource.PROJECT_MEMBERS_PAGE);
        }
    });
}

/**
 * Makes delete project members dialog visible.
 */
function showDeleteDialog(): void {
    isRemoveMembersDialogShown.value = true;
}

/**
 * Handles update table search.
 */
watch(() => search.value, () => {
    clearTimeout(searchTimer.value);

    searchTimer.value = setTimeout(() => {
        pmStore.setSearchQuery(search.value || '');
        fetch();
    }, 500); // 500ms delay for every new call.
});

onMounted(() => {
    fetch();
});
</script>

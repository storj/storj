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
        :maxlength="MAX_SEARCH_VALUE_LENGTH"
        class="mb-5"
    />

    <v-data-table-server
        v-model="selectedMembers"
        :search="search"
        :headers="headers"
        :items="projectMembers"
        :loading="isLoading"
        :items-length="page.totalCount"
        items-per-page-text="Accounts per page"
        :items-per-page-options="tableSizeOptions(page.totalCount)"
        no-data-text="No results found"
        :item-value="(item) => ({email: item.email, isInvite: hasInviteActionItem(item)})"
        select-strategy="all"
        item-selectable="selectable"
        :show-select="isUserAdmin"
        :hover="isUserAdmin"
        @update:items-per-page="onUpdateLimit"
        @update:page="onUpdatePage"
        @update:sort-by="onUpdateSortBy"
    >
        <template #item.name="{ item }">
            <span class="font-weight-bold">
                {{ item.name }}
            </span>
        </template>
        <template #item.role="{ item }">
            <v-chip :color="PROJECT_ROLE_COLORS[item.role]" variant="tonal" size="small" class="font-weight-bold">
                {{ item.role }}
            </v-chip>
        </template>
        <template #item.actions="{ item }">
            <v-btn
                v-if="hasActionMenu(item)"
                variant="outlined"
                color="default"
                size="small"
                rounded="md"
                class="mr-1 text-caption"
                density="comfortable"
                icon
            >
                <v-icon :icon="Ellipsis" />
                <v-menu activator="parent">
                    <v-list class="pa-1">
                        <v-list-item
                            v-if="hasChangeRoleActionItem(item)"
                            density="comfortable"
                            link
                            @click="() => showChangeRoleDialog(item)"
                        >
                            <template #prepend>
                                <component :is="UserCog" :size="18" />
                            </template>
                            <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                Change Role
                            </v-list-item-title>
                        </v-list-item>
                        <v-list-item
                            v-if="hasInviteActionItem(item)"
                            density="comfortable"
                            link
                            @click="() => onResendOrCopyClick(item.expired, item.email)"
                        >
                            <template #prepend>
                                <component :is="Send" v-if="item.expired" :size="18" />
                                <component :is="Copy" v-else :size="18" />
                            </template>
                            <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                {{ item.expired ? 'Resend Invite' : 'Copy Invite Link' }}
                            </v-list-item-title>
                        </v-list-item>
                        <v-divider
                            v-if="hasInviteActionItem(item) || hasChangeRoleActionItem(item)"
                            class="my-1"
                        />
                        <v-list-item
                            class="text-error"
                            density="comfortable"
                            link
                            @click="() => onSingleDelete(item)"
                        >
                            <template #prepend>
                                <component :is="UserMinus" :size="18" />
                            </template>
                            <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                {{ hasInviteActionItem(item) ? "Remove Invite" : "Remove Member" }}
                            </v-list-item-title>
                        </v-list-item>
                    </v-list>
                </v-menu>
            </v-btn>
        </template>
    </v-data-table-server>

    <remove-project-member-dialog
        v-model="isRemoveMembersDialogShown"
        :removables="membersToDelete"
        @deleted="onPostDelete"
    />

    <change-member-role-dialog
        v-model="isChangeMembersRoleShown"
        :email="memberToUpdate?.email"
        :member-id="memberToUpdate?.id"
    />

    <v-snackbar
        rounded="lg"
        variant="elevated"
        color="surface"
        :model-value="!!selectedMembers.length"
        :timeout="-1"
        class="snackbar-multiple"
    >
        <v-row align="center" justify="space-between">
            <v-col>
                {{ selectedMembers.length }} user{{ selectedMembers.length > 1 ? 's' : '' }} selected
            </v-col>
            <v-col>
                <div class="d-flex justify-end">
                    <v-btn
                        color="error"
                        density="comfortable"
                        variant="outlined"
                        @click="showDeleteDialog"
                    >
                        <template #prepend>
                            <component :is="UserMinus" :size="18" />
                        </template>
                        Remove
                    </v-btn>
                </div>
            </v-col>
        </v-row>
    </v-snackbar>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import {
    VRow,
    VCol,
    VTextField,
    VChip,
    VIcon,
    VList,
    VMenu,
    VListItem,
    VBtn,
    VListItemTitle,
    VDataTableServer,
    VSnackbar,
    VDivider,
} from 'vuetify/components';
import { useRouter } from 'vue-router';
import {
    Ellipsis,
    Search,
    Send,
    Copy,
    UserMinus,
    UserCog }
    from 'lucide-vue-next';

import { Time } from '@/utils/time';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import {
    ProjectInvitationItemModel,
    ProjectMemberCursor,
    ProjectMemberOrderBy,
    ProjectMembersPage,
    ProjectRole,
} from '@/types/projectMembers';
import { Project, PROJECT_ROLE_COLORS } from '@/types/projects';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { SortDirection, tableSizeOptions, MAX_SEARCH_VALUE_LENGTH, DataTableHeader } from '@/types/common';
import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';
import { ROUTES } from '@/router';
import { User } from '@/types/users';

import RemoveProjectMemberDialog from '@/components/dialogs/RemoveProjectMemberDialog.vue';
import ChangeMemberRoleDialog from '@/components/dialogs/ChangeMemberRoleDialog.vue';

type RenderedItem = {
    id: string | undefined,
    name: string,
    email: string,
    role: ProjectRole,
    date: string,
    selectable: boolean,
    expired: boolean,
};

const usersStore = useUsersStore();
const analyticsStore = useAnalyticsStore();
const pmStore = useProjectMembersStore();
const projectsStore = useProjectsStore();
const configStore = useConfigStore();

const router = useRouter();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const props = defineProps<{
    isUserAdmin: boolean
}>();

const isRemoveMembersDialogShown = ref<boolean>(false);
const isChangeMembersRoleShown = ref<boolean>(false);
const search = ref<string>('');
const searchTimer = ref<NodeJS.Timeout>();
const selectedMembers = ref<{ email:string, isInvite: boolean }[]>([]);
const memberToDelete = ref<{ email:string, isInvite: boolean }>();
const memberToUpdate = ref<RenderedItem>();

const headers = ref<DataTableHeader[]>([
    {
        title: 'Name',
        align: 'start',
        key: 'name',
    },
    { title: 'Email', key: 'email' },
    { title: 'Role', key: 'role', sortable: false },
    { title: 'Date Added', key: 'date' },
    { title: '', key: 'actions', sortable: false, width: 0 },
]);

const selectedProject = computed<Project>(() => projectsStore.state.selectedProject);
const user = computed<User>(() => usersStore.state.user);
const isProjectOwner = computed<boolean>(() => selectedProject.value.ownerId === user.value.id);
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
    if (projectOwner) projectMembersToReturn.unshift(projectOwner);

    return projectMembersToReturn.map<RenderedItem>(member => {
        const memberID = member.getUserID();

        let role = member.getRole();
        if (memberID && memberID === projectOwner?.getUserID()) {
            role = ProjectRole.Owner;
        } else if (member.isPending()) {
            if ((member as ProjectInvitationItemModel).expired) {
                role = ProjectRole.InviteExpired;
            } else {
                role = ProjectRole.Invited;
            }
        }

        return {
            id: memberID,
            name: member.getName(),
            email: member.getEmail(),
            role,
            date: Time.formattedDate(member.getJoinDate()),
            selectable: role !== ProjectRole.Owner,
            expired: member.isPending() && 'expired' in member && Boolean(member.expired),
        };
    });
});

/**
 * Returns the members to be deleted to the delete dialog.
 */
const membersToDelete = computed<{ email:string, isInvite: boolean }[]>(() => {
    if (memberToDelete.value) return [memberToDelete.value];
    return selectedMembers.value;
});

/**
 * Indicates if action menu should be rendered.
 */
function hasActionMenu(item: RenderedItem): boolean {
    return item.role !== ProjectRole.Owner && (props.isUserAdmin || user.value.id === item.id);
}

/**
 * Indicates if action menu should have change role item.
 */
function hasChangeRoleActionItem(item: RenderedItem): boolean {
    return isProjectOwner.value && (item.role === ProjectRole.Member || item.role === ProjectRole.Admin);
}

/**
 * Indicates if action menu should have invite-related item.
 */
function hasInviteActionItem(item: RenderedItem): boolean {
    return item.role === ProjectRole.Invited || item.role === ProjectRole.InviteExpired;
}

/**
 * Handles update table rows limit event.
 */
async function onUpdateLimit(limit: number): Promise<void> {
    await fetch(FIRST_PAGE, limit);
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
    if (selectedMembers.value.map(s => s.email).includes(usersStore.state.user.email)) {
        router.push(ROUTES.Projects.path);
        return;
    }

    search.value = '';
    selectedMembers.value = [];
    memberToDelete.value = undefined;
    await onUpdatePage(FIRST_PAGE);
}

function onSingleDelete(item: RenderedItem): void {
    memberToDelete.value = { email: item.email, isInvite: hasInviteActionItem(item) };
    isRemoveMembersDialogShown.value = true;
}

/**
 * Handles update table sorting event.
 */
async function onUpdateSortBy(sortBy: { key: keyof ProjectMemberOrderBy, order: keyof SortDirection }[]): Promise<void> {
    if (!sortBy.length) return;

    const sorting = sortBy[0];

    pmStore.setSortingBy(ProjectMemberOrderBy[sorting.key.toUpperCase()]);
    pmStore.setSortingDirection(SortDirection[sorting.order]);

    await fetch(FIRST_PAGE, cursor.value.limit);
}

/**
 * Handles on invite raw action click logic depending on expiration status.
 */
async function onResendOrCopyClick(expired: boolean, email: string): Promise<void> {
    if (expired) {
        await resendInvite(email);
    } else {
        await copyInviteLink(email);
    }
}

/**
 * Resends project invitation to current project.
 */
async function resendInvite(email: string): Promise<void> {
    await withLoading(async () => {
        analyticsStore.eventTriggered(AnalyticsEvent.RESEND_INVITE_CLICKED, { project_id: selectedProject.value.id });
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
            notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_MEMBERS_PAGE);
        }
    });
}

/**
 * Makes change project member role dialog visible.
 */
function showChangeRoleDialog(item: RenderedItem): void {
    memberToUpdate.value = item;
    isChangeMembersRoleShown.value = true;
}

/**
 * Makes delete project members dialog visible.
 */
function showDeleteDialog(): void {
    isRemoveMembersDialogShown.value = true;
}

watch(isRemoveMembersDialogShown, (value) => {
    if (!value) memberToDelete.value = undefined;
});

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

onBeforeUnmount(() => {
    pmStore.setSearchQuery('');
});
</script>

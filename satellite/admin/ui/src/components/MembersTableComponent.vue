// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="flat" border rounded="xlg">
        <v-data-table-server
            :headers="headers"
            :items="items"
            :search="search"
            :loading="isLoading"
            :items-length="membersPage?.totalCount ?? 0"
            :items-per-page="pageSize"
            :items-per-page-options="itemsPerPageOptions"
            items-per-page-text="Members per page"
            no-data-text="No members found"
            class="border-0"
            hover
            @update:items-per-page="onUpdateLimit"
            @update:page="onUpdatePage"
            @update:sort-by="onUpdateSortBy"
        >
            <template #top>
                <v-text-field
                    v-model="search" label="Search" :prepend-inner-icon="Search" single-line variant="solo-filled" flat
                    hide-details clearable density="compact" rounded="lg" class="mx-2 my-2"
                />
            </template>
            <template #item.email="{ item }">
                <div class="text-no-wrap">
                    <v-chip
                        v-tooltip="'Go to account'"
                        variant="text"
                        color="default"
                        size="small"
                        link
                        class="font-weight-bold pl-1 ml-1"
                        @click="goToAccount(item.userID)"
                    >
                        <template #prepend>
                            <svg class="mr-2" width="24" height="24" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <rect x="0.5" y="0.5" width="31" height="31" rx="10" stroke="currentColor" stroke-opacity="0.2" />
                                <path
                                    d="M16.0695 8.34998C18.7078 8.34998 20.8466 10.4888 20.8466 13.1271C20.8466 14.7102 20.0765 16.1134 18.8905 16.9826C21.2698 17.8565 23.1437 19.7789 23.9536 22.1905C23.9786 22.265 24.0026 22.34 24.0256 22.4154L24.0593 22.5289C24.2169 23.0738 23.9029 23.6434 23.3579 23.801C23.2651 23.8278 23.1691 23.8414 23.0725 23.8414H8.91866C8.35607 23.8414 7.89999 23.3853 7.89999 22.8227C7.89999 22.7434 7.90926 22.6644 7.92758 22.5873L7.93965 22.5412C7.97276 22.4261 8.00827 22.3119 8.04612 22.1988C8.86492 19.7523 10.7783 17.8081 13.2039 16.9494C12.0432 16.0781 11.2924 14.6903 11.2924 13.1271C11.2924 10.4888 13.4312 8.34998 16.0695 8.34998ZM16.0013 17.9724C13.1679 17.9724 10.6651 19.7017 9.62223 22.264L9.59178 22.34H22.4107L22.4102 22.3388C21.3965 19.7624 18.9143 18.0092 16.0905 17.973L16.0013 17.9724ZM16.0695 9.85135C14.2604 9.85135 12.7938 11.3179 12.7938 13.1271C12.7938 14.9362 14.2604 16.4028 16.0695 16.4028C17.8786 16.4028 19.3452 14.9362 19.3452 13.1271C19.3452 11.3179 17.8786 9.85135 16.0695 9.85135Z"
                                    fill="currentColor"
                                />
                            </svg>
                        </template>
                        {{ item.email }}
                    </v-chip>
                </div>
            </template>

            <template #item.role="{ item }">
                <v-chip variant="tonal" :color="getRoleColor(item)" size="small">
                    {{ getRole(item) }}
                </v-chip>
            </template>

            <template #item.createdAt="{ item }">
                <span class="text-no-wrap">
                    {{ date.format(item.createdAt, 'fullDate') }}
                </span>
            </template>
        </v-data-table-server>
    </v-card>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { VCard, VChip, VDataTableServer, VTextField } from 'vuetify/components';
import { Search } from 'lucide-vue-next';
import { useDate } from 'vuetify/framework';
import { useRouter } from 'vue-router';

import { DataTableHeader } from '@/types/common';
import { useLoading } from '@/composables/useLoading';
import { Project, ProjectMember, ProjectMembersPage } from '@/api/client.gen';
import { useProjectsStore } from '@/store/projects';
import { useNotify } from '@/composables/useNotify';
import { ProjectMemberOrderBy, ProjectMemberOrderDirection, ProjectMemberRole } from '@/types/project';
import { ROUTES } from '@/router';

const date = useDate();
const { isLoading, withLoading } = useLoading();
const notify = useNotify();
const router = useRouter();

const projectsStore = useProjectsStore();

const props = defineProps<{
    project: Project;
}>();

const search = ref<string>('');
const pageSize = ref<number>(10);
const searchTimer = ref<NodeJS.Timeout>();
const membersPage = ref<ProjectMembersPage>();
const orderBy = ref<ProjectMemberOrderBy>(ProjectMemberOrderBy.EMAIL);
const orderDirection = ref<ProjectMemberOrderDirection>(ProjectMemberOrderDirection.ASC);

const headers: DataTableHeader[] = [
    { title: 'Email', key: 'email' },
    { title: 'User ID', key: 'userID', sortable: false },
    { title: 'Role', key: 'role', sortable: false },
    { title: 'Date Added', key: 'createdAt' },
];

const itemsPerPageOptions = [
    { value: 10, title: '10' },
    { value: 25, title: '25' },
    { value: 50, title: '50' },
    { value: 100, title: '100' },
];

const items = computed<ProjectMember[]>(() => {
    if (!(membersPage.value && membersPage.value.projectMembers)) return [];

    const projectOwner = membersPage.value.projectMembers.find((member) => member.userID === props.project.owner.id);
    const projectMembersToReturn = membersPage.value.projectMembers.filter((member) => member.userID !== props.project.owner.id);

    // if the project owner exists, place at the front of the members list.
    if (projectOwner) projectMembersToReturn.unshift(projectOwner);

    return projectMembersToReturn;
});

function getRole(item: ProjectMember): string {
    if (props.project.owner.id === item.userID) return 'Owner';

    return item.role === ProjectMemberRole.ADMIN ? 'Admin' : 'Member';
}

function getRoleColor(item: ProjectMember): string {
    if (props.project.owner.id === item.userID) return 'primary';

    return item.role === ProjectMemberRole.ADMIN ? 'purple' : 'success';
}

/**
 * Handles update table rows limit event.
 */
function onUpdateLimit(limit: number): void {
    pageSize.value = limit;
    fetchMembers(1, limit);
}

/**
 * Handles update table page event.
 */
function onUpdatePage(page: number): void {
    fetchMembers(page, pageSize.value);
}

function onUpdateSortBy(sortBy: { key: keyof ProjectMember, order: 'asc' | 'desc' }[]): void {
    if (!sortBy.length) return;

    const sorting = sortBy[0];

    orderBy.value = sorting.key === 'email' ? ProjectMemberOrderBy.EMAIL : ProjectMemberOrderBy.DATE;
    orderDirection.value = sorting.order === 'asc' ? ProjectMemberOrderDirection.ASC : ProjectMemberOrderDirection.DESC;

    fetchMembers(1, pageSize.value);
}

/**
 * Fetches bucket using api.
 */
function fetchMembers(page = 1, limit = 10): void {
    withLoading(async () => {
        try {
            membersPage.value = await projectsStore.getProjectMembers(props.project.id, {
                search: search.value,
                page, limit,
                order: orderBy.value,
                orderDirection: orderDirection.value,
            });
        } catch (error) {
            notify.error(`Failed to fetch project members: ${error.message}`);
        }
    });
}

function goToAccount(id: string): void {
    router.push({ name: ROUTES.Account.name, params: { userID: id } });
}

watch(search, () => {
    clearTimeout(searchTimer.value);

    searchTimer.value = setTimeout(() => {
        fetchMembers();
    }, 500); // 500ms delay for every new call.
});

onMounted(() => {
    fetchMembers();
});
</script>
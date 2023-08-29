// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="flat" :border="true" rounded="xlg">
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
            class="mx-2 mt-2"
        />

        <v-data-table
            v-model="selected"
            :sort-by="sortBy"
            :headers="headers"
            :items="projectMembers"
            :search="search"
            class="elevation-1"
            item-value="email"
            show-select
            hover
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
        </v-data-table>
    </v-card>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { VCard, VTextField, VChip } from 'vuetify/components';
import { VDataTable } from 'vuetify/labs/components';

import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { ProjectInvitationItemModel, ProjectRole } from '@/types/projectMembers';
import { PROJECT_ROLE_COLORS } from '@poc/types/projects';

const pmStore = useProjectMembersStore();
const projectsStore = useProjectsStore();

const search = ref<string>('');
const selected = ref([]);
const sortBy = ref([{ key: 'date', order: 'asc' }]);
const headers = ref([
    {
        title: 'Name',
        align: 'start',
        key: 'name',
    },
    { title: 'Email', key: 'email' },
    { title: 'Role', key: 'role' },
    { title: 'Date Added', key: 'date' },
]);

/**
 * Returns team members of current page from store.
 * With project owner pinned to top
 */
const projectMembers = computed(() => {
    const projectMembers = pmStore.state.page.getAllItems();
    const projectOwner = projectMembers.find((member) => member.getUserID() === projectsStore.state.selectedProject.ownerId);
    const projectMembersToReturn = projectMembers.filter((member) => member.getUserID() !== projectsStore.state.selectedProject.ownerId);

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
        };
    });
});
</script>

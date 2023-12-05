// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-table
        :total-items-count="projects.length + invites?.length || 0"
        class="projects-table"
        items-label="projects"
    >
        <template #head>
            <th class="align-left">Project</th>
            <th class="date-added align-left">Date Added</th>
            <th class="members align-left">Members</th>
            <th class="role align-left">Role</th>
        </template>
        <template #body>
            <project-table-invitation-item
                v-for="(invite, key) in invites"
                :key="key"
                :invitation="invite"
            />
            <project-table-item
                v-for="(project, key) in projects"
                :key="key"
                :project="project"
            />
        </template>
    </v-table>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { Project, ProjectInvitation } from '@/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';

import ProjectTableItem from '@/views/all-dashboard/components/ProjectTableItem.vue';
import ProjectTableInvitationItem from '@/views/all-dashboard/components/ProjectTableInvitationItem.vue';
import VTable from '@/components/common/VTable.vue';

const projectsStore = useProjectsStore();

const props = withDefaults(defineProps<{
    invites?: ProjectInvitation[],
}>(), {
    invites: () => [],
});

/**
 * Returns projects list from store.
 */
const projects = computed((): Project[] => {
    return projectsStore.projects;
});

</script>

<style scoped lang="scss">
.projects-table {
    @media screen and (width <= 550px) {

        :deep(.table-footer), :deep(.base-table) {
            border-radius: 0;
        }
    }
}

@media screen and (width <= 850px) {

    .date-added, .members {
        display: none;
    }
}

@media screen and (width <= 600px) {

    .role {
        display: none;
    }
}
</style>
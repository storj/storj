// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-table
        :total-items-count="projects.length"
        class="projects-table"
        items-label="projects"
    >
        <template #head>
            <th class="sort-header-container__name-item align-left">Project</th>
            <th class="sort-header-container__date-item align-left">Date Added</th>
            <th class="sort-header-container__date-item align-left">Members</th>
            <th class="sort-header-container__date-item align-left">Role</th>
        </template>
        <template #body>
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

import { Project } from '@/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';
import ProjectTableItem from '@/views/all-dashboard/components/ProjectTableItem.vue';

import VTable from '@/components/common/VTable.vue';

const projectsStore = useProjectsStore();

/**
 * Returns projects list from store.
 */
const projects = computed((): Project[] => {
    return projectsStore.projects;
});

</script>

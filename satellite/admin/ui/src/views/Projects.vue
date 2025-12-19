// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container fluid>
        <v-row>
            <v-col cols="6">
                <PageTitleComponent title="Projects" />
                <PageSubtitleComponent subtitle="All projects on North America US1." />
            </v-col>

            <v-col cols="6" class="d-flex justify-end align-center">
                <v-btn variant="outlined" color="default">
                    <svg width="16" height="16" class="mr-2" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path
                            d="M10 1C14.9706 1 19 5.02944 19 10C19 14.9706 14.9706 19 10 19C5.02944 19 1 14.9706 1 10C1 5.02944 5.02944 1 10 1ZM10 2.65C5.94071 2.65 2.65 5.94071 2.65 10C2.65 14.0593 5.94071 17.35 10 17.35C14.0593 17.35 17.35 14.0593 17.35 10C17.35 5.94071 14.0593 2.65 10 2.65ZM10.7496 6.8989L10.7499 6.91218L10.7499 9.223H12.9926C13.4529 9.223 13.8302 9.58799 13.8456 10.048C13.8602 10.4887 13.5148 10.8579 13.0741 10.8726L13.0608 10.8729L10.7499 10.873L10.75 13.171C10.75 13.6266 10.3806 13.996 9.925 13.996C9.48048 13.996 9.11807 13.6444 9.10066 13.2042L9.1 13.171L9.09985 10.873H6.802C6.34637 10.873 5.977 10.5036 5.977 10.048C5.977 9.60348 6.32857 9.24107 6.76882 9.22366L6.802 9.223H9.09985L9.1 6.98036C9.1 6.5201 9.46499 6.14276 9.925 6.12745C10.3657 6.11279 10.7349 6.45818 10.7496 6.8989Z"
                            fill="currentColor"
                        />
                    </svg>
                    New Project
                    <NewProjectDialog />
                </v-btn>
            </v-col>
        </v-row>

        <ProjectsTableComponent class="my-5" />
    </v-container>
</template>

<script setup lang="ts">
import { VContainer, VRow, VCol, VBtn } from 'vuetify/components';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import ProjectsTableComponent from '@/components/ProjectsTableComponent.vue';
import NewProjectDialog from '@/components/NewProjectDialog.vue';
</script>
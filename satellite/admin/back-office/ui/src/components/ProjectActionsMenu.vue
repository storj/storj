// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-menu activator="parent">
        <v-list class="pa-2">
            <v-list-item density="comfortable" link rounded="lg" base-color="info" router-link to="/project-details">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    View Project
                </v-list-item-title>
            </v-list-item>

            <v-divider class="my-2" />

            <v-list-item density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Edit Project
                    <ProjectInformationDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Set Value
                    <ProjectUserAgentsDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Set Placement
                    <ProjectGeofenceDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Change Limits
                    <ProjectLimitsDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    New Bucket
                    <ProjectNewBucketDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Add User
                    <ProjectAddUserDialog />
                </v-list-item-title>
            </v-list-item>

            <v-divider class="my-2" />
            <v-list-item density="comfortable" link rounded="lg" base-color="error">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Delete
                    <ProjectDeleteDialog />
                </v-list-item-title>
            </v-list-item>
        </v-list>
    </v-menu>
</template>

<script lang="ts">
import { VMenu, VList, VListItem, VListItemTitle, VDivider } from 'vuetify/components';

import ProjectInformationDialog from '@/components/ProjectInformationDialog.vue';
import ProjectDeleteDialog from '@/components/ProjectDeleteDialog.vue';
import ProjectNewBucketDialog from '@/components/ProjectNewBucketDialog.vue';
import ProjectGeofenceDialog from '@/components/ProjectGeofenceDialog.vue';
import ProjectUserAgentsDialog from '@/components/ProjectUserAgentsDialog.vue';
import ProjectLimitsDialog from '@/components/ProjectLimitsDialog.vue';
import ProjectAddUserDialog from '@/components/ProjectAddUserDialog.vue';

export default {
    name: 'ProjectActionsMenu',
    components: {
        VMenu,
        VList,
        VListItem,
        VListItemTitle,
        VDivider,
        ProjectDeleteDialog,
        ProjectNewBucketDialog,
        ProjectGeofenceDialog,
        ProjectUserAgentsDialog,
        ProjectLimitsDialog,
        ProjectAddUserDialog,
        ProjectInformationDialog,
    },
};
</script>

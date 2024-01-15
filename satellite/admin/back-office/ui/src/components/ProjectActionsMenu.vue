// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-menu activator="parent">
        <v-list class="pa-2">
            <v-list-item v-if="featureFlags.project.view" density="comfortable" link rounded="lg" base-color="info" router-link to="/project-details">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    View Project
                </v-list-item-title>
            </v-list-item>

            <v-divider class="my-2" />

            <v-list-item v-if="featureFlags.project.updateInfo" density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Edit Project
                    <ProjectInformationDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item v-if="featureFlags.project.updateValueAttribution" density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Set Value
                    <ProjectUserAgentsDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item v-if="featureFlags.project.upatePlacement" density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Set Placement
                    <ProjectGeofenceDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item v-if="featureFlags.project.updateLimits" density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Change Limits
                    <ProjectLimitsDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item v-if="featureFlags.bucket.create" density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    New Bucket
                    <ProjectNewBucketDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item v-if="featureFlags.project.memberAdd" density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Add User
                    <ProjectAddUserDialog />
                </v-list-item-title>
            </v-list-item>

            <v-divider class="my-2" />
            <v-list-item v-if="featureFlags.project.delete" density="comfortable" link rounded="lg" base-color="error">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Delete
                    <ProjectDeleteDialog />
                </v-list-item-title>
            </v-list-item>
        </v-list>
    </v-menu>
</template>

<script setup lang="ts">
import { VMenu, VList, VListItem, VListItemTitle, VDivider } from 'vuetify/components';

import { FeatureFlags } from '@/api/client.gen';
import { useAppStore } from '@/store/app';

import ProjectInformationDialog from '@/components/ProjectInformationDialog.vue';
import ProjectDeleteDialog from '@/components/ProjectDeleteDialog.vue';
import ProjectNewBucketDialog from '@/components/ProjectNewBucketDialog.vue';
import ProjectGeofenceDialog from '@/components/ProjectGeofenceDialog.vue';
import ProjectUserAgentsDialog from '@/components/ProjectUserAgentsDialog.vue';
import ProjectLimitsDialog from '@/components/ProjectLimitsDialog.vue';
import ProjectAddUserDialog from '@/components/ProjectAddUserDialog.vue';

const featureFlags = useAppStore().state.settings.admin.features as FeatureFlags;
</script>

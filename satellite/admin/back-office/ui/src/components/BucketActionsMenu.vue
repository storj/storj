// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-menu activator="parent">
        <v-list class="pa-2">
            <v-list-item v-if="featureFlags.bucket.view" density="comfortable" link rounded="lg" base-color="info" router-link to="/bucket-details">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    View Bucket
                </v-list-item-title>
            </v-list-item>

            <v-divider v-if="featureFlags.bucket.updateInfo || featureFlags.bucket.updateValueAttribution || featureFlags.bucket.updatePlacement" class="my-2" />

            <v-list-item v-if="featureFlags.bucket.updateInfo" density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Edit Bucket
                    <BucketInformationDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item v-if="featureFlags.bucket.updateValueAttribution" density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Set Value
                    <BucketUserAgentsDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item v-if="featureFlags.bucket.updatePlacement" density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Set Placement
                    <BucketGeofenceDialog />
                </v-list-item-title>
            </v-list-item>

            <v-divider v-if="featureFlags.bucket.delete" class="my-2" />

            <v-list-item v-if="featureFlags.bucket.delete" density="comfortable" link rounded="lg" base-color="error">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Delete Bucket
                    <BucketDeleteDialog />
                </v-list-item-title>
            </v-list-item>
        </v-list>
    </v-menu>
</template>

<script setup lang="ts">
import { VMenu, VList, VListItem, VListItemTitle, VDivider } from 'vuetify/components';

import { FeatureFlags } from '@/api/client.gen';
import { useAppStore } from '@/store/app';

import BucketInformationDialog from '@/components/BucketInformationDialog.vue';
import BucketDeleteDialog from '@/components/BucketDeleteDialog.vue';
import BucketGeofenceDialog from '@/components/BucketGeofenceDialog.vue';
import BucketUserAgentsDialog from '@/components/BucketUserAgentsDialog.vue';

const featureFlags = useAppStore().state.settings.admin.features as FeatureFlags;
</script>

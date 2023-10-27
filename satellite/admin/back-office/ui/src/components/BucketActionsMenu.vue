// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-menu activator="parent">
        <v-list class="pa-2">
            <v-list-item density="comfortable" link rounded="lg" base-color="info" router-link to="/bucket-details">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    View Bucket
                </v-list-item-title>
            </v-list-item>

            <v-divider class="my-2" />

            <v-list-item density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Edit Bucket
                    <BucketInformationDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Set Value
                    <BucketUserAgentsDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Set Placement
                    <BucketGeofenceDialog />
                </v-list-item-title>
            </v-list-item>

            <v-divider class="my-2" />

            <v-list-item density="comfortable" link rounded="lg" base-color="error">
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

import BucketInformationDialog from '@/components/BucketInformationDialog.vue';
import BucketDeleteDialog from '@/components/BucketDeleteDialog.vue';
import BucketGeofenceDialog from '@/components/BucketGeofenceDialog.vue';
import BucketUserAgentsDialog from '@/components/BucketUserAgentsDialog.vue';
</script>

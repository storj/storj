// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-7">
        <v-row>
            <v-col>
                <p class="font-weight-bold mb-2">Select Buckets</p>
                <p>Choose buckets you want this application to access.</p>
                <v-chip-group
                    filter
                    selected-class="text-primary font-weight-bold"
                    class="mt-2 mb-3"
                >
                    <v-chip
                        variant="outlined"
                        filter
                        value="all"
                        color="default"
                    >
                        All Buckets
                    </v-chip>

                    <v-chip
                        variant="outlined"
                        filter
                        color="info"
                        value="select"
                    >
                        Select Buckets
                    </v-chip>

                    <v-chip
                        variant="outlined"
                        filter
                        color="success"
                        value="new"
                    >
                        New Bucket
                    </v-chip>
                </v-chip-group>

                <v-alert v-if="buckets === 'all'" variant="tonal" color="default">
                    <p class="text-subtitle-2 font-weight-bold">All Buckets</p>
                    <p class="text-subtitle-2">The application can access all of the current and future buckets you create in this project.</p>
                </v-alert>

                <v-alert v-else-if="buckets === 'select'" variant="tonal" color="info">
                    <p class="text-subtitle-2 font-weight-bold">Select specific buckets</p>
                    <p class="text-subtitle-2">Please select which buckets do you want to share.</p>
                    <v-select label="Select Buckets" density="comfortable" class="mt-3" hide-details />
                </v-alert>

                <v-alert v-else variant="tonal" color="success">
                    <p class="text-subtitle-2 font-weight-bold">Create a new bucket</p>
                    <p class="text-subtitle-2">New bucket for this access only.</p>
                    <v-text-field label="Enter a bucket name" density="comfortable" class="mt-3" hide-details />
                </v-alert>
            </v-col>
        </v-row>
    </v-form>
</template>

<script setup lang="ts">
import { VForm, VAlert, VChip, VChipGroup, VCol, VRow, VSelect, VTextField } from 'vuetify/components';
</script>

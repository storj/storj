// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-7">
        <v-row>
            <v-col cols="12">
                <p class="font-weight-bold mb-2">
                    Choose Permissions
                </p>
                <p>Select which permissions to give this application.</p>
                <v-chip-group
                    filter
                    multiple
                    selected-class="text-primary font-weight-bold"
                    class="mt-2 mb-3"
                >
                    <v-chip
                        key="all"
                        variant="outlined"
                        filter
                        value="all"
                        color="default"
                    >
                        All
                    </v-chip>

                    <v-chip
                        key="read"
                        variant="outlined"
                        filter
                        value="read"
                        color="secondary"
                    >
                        Read
                    </v-chip>

                    <v-chip
                        key="write"
                        variant="outlined"
                        filter
                        value="write"
                        color="green"
                    >
                        Write
                    </v-chip>

                    <v-chip
                        key="list"
                        variant="outlined"
                        filter
                        value="list"
                        color="help"
                    >
                        List
                    </v-chip>

                    <v-chip
                        key="delete"
                        variant="outlined"
                        filter
                        value="delete"
                        color="warning"
                    >
                        Delete
                    </v-chip>
                </v-chip-group>
                <v-alert variant="tonal" color="info" width="auto">
                    <p class="text-subtitle-2 font-weight-bold">
                        Important
                    </p>
                    <p class="text-subtitle-2">
                        If you don't select the correct permissions, your application might not connect properly.
                    </p>
                </v-alert>
            </v-col>
        </v-row>
    </v-form>
</template>

<script setup lang="ts">
import { VForm, VAlert, VChip, VChipGroup, VCol, VRow } from 'vuetify/components';
</script>

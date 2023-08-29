// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-menu activator="parent">
        <v-list class="pa-2">
            <v-list-item density="comfortable" link rounded="lg">
                <template #prepend>
                    <icon-preview />
                </template>
                <v-list-item-title class="pl-2 text-body-2 font-weight-medium">
                    Preview
                </v-list-item-title>
            </v-list-item>

            <v-list-item density="comfortable" link rounded="lg">
                <template #prepend>
                    <icon-download />
                </template>
                <v-list-item-title class="pl-2 text-body-2 font-weight-medium">
                    Download
                </v-list-item-title>
            </v-list-item>

            <v-list-item density="comfortable" link rounded="lg">
                <template #prepend>
                    <icon-share bold />
                </template>
                <v-list-item-title class="pl-2 text-body-2 font-weight-medium">
                    Share
                </v-list-item-title>
            </v-list-item>

            <v-divider class="my-2" />

            <v-list-item density="comfortable" link rounded="lg" base-color="error">
                <template #prepend>
                    <icon-trash bold />
                </template>
                <v-list-item-title class="pl-2 text-body-2 font-weight-medium">
                    Delete
                </v-list-item-title>
            </v-list-item>
        </v-list>
    </v-menu>
</template>

<script setup lang="ts">
import { VMenu, VList, VListItem, VListItemTitle, VDivider } from 'vuetify/components';

import IconDownload from '@poc/components/icons/IconDownload.vue';
import IconShare from '@poc/components/icons/IconShare.vue';
import IconPreview from '@poc/components/icons/IconPreview.vue';
import IconTrash from '@poc/components/icons/IconTrash.vue';
</script>

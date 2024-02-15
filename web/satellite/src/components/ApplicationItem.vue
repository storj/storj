// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-col cols="12" md="6" lg="6" xl="3">
        <v-card class="px-2 py-4">
            <v-card-item class="pb-0">
                <img :src="app.src" :alt="app.title" width="42" class="rounded">
            </v-card-item>

            <v-card-item>
                <v-chip size="small" variant="tonal" color="default" class="mb-3 font-weight-medium" rounded>
                    {{ app.category }}
                </v-chip>
                <h3 class="mb-1">
                    {{ app.title }}
                </h3>
                <p class="mt-1 text-medium-emphasis">
                    {{ app.description }}
                </p>
            </v-card-item>

            <v-card-item class="bottom">
                <v-btn color="primary">
                    Setup
                    <!-- TODO: add create access grant flow -->
                    <template #append>
                        <v-icon :icon="mdiArrowRight" />
                    </template>
                </v-btn>
                <v-btn variant="outlined" color="default" class="ml-2" :href="app.docs" target="_blank" rel="noopener noreferrer">
                    Docs
                    <template #append>
                        <v-icon :icon="mdiOpenInNew" />
                    </template>
                </v-btn>
            </v-card-item>
        </v-card>
    </v-col>
</template>

<script setup lang="ts">
import { VCol, VCard, VCardItem, VChip, VIcon, VBtn } from 'vuetify/components';
import { mdiArrowRight, mdiOpenInNew } from '@mdi/js';

import { Application } from '@/types/applications';

const props = defineProps<{
    app: Application
}>();
</script>

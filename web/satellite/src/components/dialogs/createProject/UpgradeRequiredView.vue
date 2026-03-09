// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card-item class="pa-6">
        <template #prepend>
            <v-sheet
                class="border-sm d-flex justify-center align-center"
                width="40"
                height="40"
                rounded="lg"
            >
                <component :is="Box" :size="18" />
            </v-sheet>
        </template>

        <v-card-title class="font-weight-bold">Create Your Own Project</v-card-title>

        <template #append>
            <v-btn
                :icon="X"
                variant="text"
                size="small"
                color="default"
                @click="emit('cancel')"
            />
        </template>
    </v-card-item>

    <v-divider />

    <v-row class="pa-6">
        <v-col>
            Your account is currently a Member account with access to shared projects.
            To create your own project, you'll need to start a free trial or upgrade to a Pro account.
        </v-col>
    </v-row>

    <v-divider />

    <v-card-actions class="pa-6">
        <v-row>
            <v-col>
                <v-btn variant="outlined" color="default" block @click="emit('cancel')">
                    Cancel
                </v-btn>
            </v-col>
            <v-col>
                <v-btn color="primary" variant="flat" block @click="emit('show-upgrade')">
                    Update Account
                </v-btn>
            </v-col>
        </v-row>
    </v-card-actions>
</template>

<script setup lang="ts">
import { VBtn, VCardActions, VCardItem, VCardTitle, VCol, VDivider, VRow, VSheet } from 'vuetify/components';
import { Box, X } from 'lucide-vue-next';

const emit = defineEmits<{
    cancel: [];
    'show-upgrade': [];
}>();
</script>

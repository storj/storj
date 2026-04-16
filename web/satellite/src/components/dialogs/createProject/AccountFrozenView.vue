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
                <component :is="LockKeyhole" :size="18" />
            </v-sheet>
        </template>

        <v-card-title class="font-weight-bold">Account Frozen</v-card-title>

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
            Your account is currently frozen. Please contact
            <a :href="configStore.supportUrl" target="_blank" rel="noopener noreferrer">support</a>
            to resolve any issues and regain access.
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
                <v-btn
                    color="primary"
                    variant="flat"
                    block
                    :href="configStore.supportUrl"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    Contact Support
                </v-btn>
            </v-col>
        </v-row>
    </v-card-actions>
</template>

<script setup lang="ts">
import { VBtn, VCardActions, VCardItem, VCardTitle, VCol, VDivider, VRow, VSheet } from 'vuetify/components';
import { LockKeyhole, X } from 'lucide-vue-next';

import { useConfigStore } from '@/store/modules/configStore';

const emit = defineEmits<{
    cancel: [];
}>();

const configStore = useConfigStore();
</script>

// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card :title="isRate ? 'Rate Limits': 'Burst Limits'" variant="flat" rounded="xlg" class="h-100">
        <template v-if="featureFlags.project.updateLimits" #append>
            <v-btn
                size="small"
                variant="outlined"
                color="default"
                @click="emit('updateLimits')"
            >
                Change Limits
            </v-btn>
        </template>
        <v-card-text>
            <div class="mt-5 d-flex justify-space-between align-center">
                <div>
                    <p class="text-medium-emphasis">General</p>
                    <p class="text-medium-emphasis">Head</p>
                    <p class="text-medium-emphasis">Get</p>
                    <p class="text-medium-emphasis">List</p>
                    <p class="text-medium-emphasis">Put</p>
                    <p class="text-medium-emphasis">Delete</p>
                </div>
                <div v-if="isRate">
                    <p>{{ project.rateLimit || '__' }}</p>
                    <p>{{ project.rateLimitHead || '__' }}</p>
                    <p>{{ project.rateLimitGet || '__' }}</p>
                    <p>{{ project.rateLimitList || '__' }}</p>
                    <p>{{ project.rateLimitPut || '__' }}</p>
                    <p>{{ project.rateLimitDelete || '__' }}</p>
                </div>
                <div v-else>
                    <p>{{ project.burstLimit || '__' }}</p>
                    <p>{{ project.burstLimitHead || '__' }}</p>
                    <p>{{ project.burstLimitGet || '__' }}</p>
                    <p>{{ project.burstLimitList || '__' }}</p>
                    <p>{{ project.burstLimitPut || '__' }}</p>
                    <p>{{ project.burstLimitDelete || '__' }}</p>
                </div>
            </div>
        </v-card-text>
    </v-card>
</template>

<script setup lang="ts">
import { VBtn, VCard, VCardText } from 'vuetify/components';

import { FeatureFlags, Project } from '@/api/client.gen';
import { useAppStore } from '@/store/app';

const featureFlags = useAppStore().state.settings.admin.features as FeatureFlags;

defineProps<{
    isRate: boolean,
    project: Project
}>();

const emit = defineEmits<{
    (e: 'updateLimits'): void;
}>();
</script>

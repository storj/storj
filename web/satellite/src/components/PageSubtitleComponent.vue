// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <p class="text-high-emphasis">
        {{ subtitle }}
        <a
            v-if="link"
            :href="link"
            target="_blank"
            rel="noopener noreferrer"
            class="link d-inline-flex align-center text-decoration-none"
        >
            <v-tooltip location="top" text="View Documentation">
                <template #activator="{ props }">
                    <component
                        v-bind="props"
                        :is="BookText"
                        :size="16"
                        class="pt-1"
                    />
                </template>
            </v-tooltip>
        </a>
    </p>
</template>

<script setup lang="ts">
import {
    VTooltip,
} from 'vuetify/components';
import { BookText } from 'lucide-vue-next';

defineProps<{
    subtitle: string;
    link?: string;
}>();
</script>

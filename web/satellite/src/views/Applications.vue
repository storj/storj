// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="pb-12">
        <v-row>
            <v-col>
                <PageTitleComponent title="Applications" />
                <PageSubtitleComponent subtitle="Connect Storj with third-party applications." link="https://www.storj.io/integrations" />
                <v-chip-group
                    v-model="selectedChips"
                    class="border rounded-xl px-2 mt-4"
                    selected-class="font-weight-bold v-chip--variant-tonal"
                    color="info"
                    mandatory
                    column
                >
                    <v-chip
                        v-for="category in categories"
                        :key="category"
                        :value="category"
                        color="info"
                        variant="text"
                        class="font-weight-medium"
                        rounded
                        filter
                    >
                        {{ category }}
                    </v-chip>
                </v-chip-group>
            </v-col>
        </v-row>

        <v-row>
            <ApplicationItem
                v-for="app in filteredApps"
                :key="app.title"
                :app="app"
            />
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { VContainer, VRow, VCol, VChipGroup, VChip } from 'vuetify/components';

import { AppCategory, Application, applications } from '@/types/applications';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import ApplicationItem from '@/components/ApplicationItem.vue';

const selectedChips = ref<AppCategory[]>([AppCategory.All]);

/**
 * Returns all application categories.
 */
const categories = computed<string[]>(() => {
    return Object.values(AppCategory);
});

/**
 * Returns filtered apps based on selected category.
 */
const filteredApps = computed<Application[]>(() => {
    if (selectedChips.value.includes(AppCategory.All)) {
        return applications;
    }

    return applications.filter(app => {
        return selectedChips.value.includes(app.category);
    });
});
</script>

// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="pb-12">
        <v-row>
            <v-col>
                <trial-expiration-banner v-if="isTrialExpirationBanner" :expired="isExpired" />

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

        <v-card class="pa-2 my-5">
            <v-row align="center">
                <v-col>
                    <v-text-field
                        v-model="search"
                        label="Search"
                        :prepend-inner-icon="mdiMagnify"
                        single-line
                        variant="solo-filled"
                        flat
                        hide-details
                        clearable
                        density="comfortable"
                        rounded="lg"
                    />
                </v-col>
                <v-col cols="auto">
                    <v-menu>
                        <template #activator="{ props }">
                            <v-btn
                                variant="text"
                                color="default"
                                :prepend-icon="mdiSort"
                                :append-icon="mdiChevronDown"
                                v-bind="props"
                                class="mr-2 ml-n2"
                                title="Sort by"
                            >
                                <span class="text-body-2 hidden-xs">Sort by</span> <span class="ml-1 text-capitalize">{{ sortKey }}</span>
                            </v-btn>
                        </template>
                        <v-list>
                            <v-list-item
                                v-for="(key, index) in sortKeys"
                                :key="index"
                                :title="key"
                                @click="() => sortKey = key.toLowerCase()"
                            />
                        </v-list>
                    </v-menu>
                    <v-btn-toggle
                        v-model="sortOrder"
                        density="comfortable"
                        variant="outlined"
                        color="default"
                        rounded="xl"
                        class="pa-1"
                        mandatory
                    >
                        <v-btn size="small" value="asc" title="Ascending" variant="text" rounded="xl">
                            <v-icon :icon="mdiSortAscending" />
                        </v-btn>
                        <v-btn size="small" value="desc" title="Descending" variant="text" rounded="xl">
                            <v-icon :icon="mdiSortDescending" />
                        </v-btn>
                    </v-btn-toggle>
                </v-col>
            </v-row>
        </v-card>

        <v-data-iterator
            :items="filteredApps"
            :items-per-page="-1"
            :search="search"
            :sort-by="sortBy"
        >
            <template #no-data>
                <div class="d-flex justify-center">
                    <p class="text-body-2">No data found</p>
                </div>
            </template>
            <template #default="{ items }">
                <v-row>
                    <ApplicationItem v-for="app in items" :key="app.raw.name" :app="app.raw" />
                </v-row>
            </template>
        </v-data-iterator>
    </v-container>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import {
    VContainer,
    VRow,
    VCol,
    VChipGroup,
    VChip,
    VCard,
    VList,
    VIcon,
    VBtn,
    VBtnToggle,
    VMenu,
    VTextField,
    VListItem,
    VDataIterator,
} from 'vuetify/components';
import { mdiChevronDown, mdiMagnify, mdiSort, mdiSortAscending, mdiSortDescending } from '@mdi/js';

import { AppCategory, Application, applications } from '@/types/applications';
import { useTrialCheck } from '@/composables/useTrialCheck';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import ApplicationItem from '@/components/ApplicationItem.vue';
import TrialExpirationBanner from '@/components/TrialExpirationBanner.vue';

const { isTrialExpirationBanner, isExpired } = useTrialCheck();

const selectedChips = ref<AppCategory[]>([AppCategory.All]);
const search = ref<string>('');
const sortKey = ref<string>('name');
const sortOrder = ref<string>('asc');
const sortKeys = ['Name', 'Category'];

/**
 * The sorting criteria to be used for the file list.
 */
const sortBy = computed(() => [{ key: sortKey.value, order: sortOrder.value }]);

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

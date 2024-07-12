// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="pb-12">
        <v-row>
            <v-col>
                <trial-expiration-banner v-if="isTrialExpirationBanner && isUserProjectOwner" :expired="isExpired" />

                <PageTitleComponent title="Applications" />
                <PageSubtitleComponent subtitle="Connect Storj with third-party applications." link="https://www.storj.io/integrations" />
                <v-chip-group
                    v-model="selectedChip"
                    class="border rounded-xlg px-2 mt-4 bg-surface"
                    selected-class="font-weight-bold v-chip--variant-tonal"
                    color="info"
                    mandatory
                    :max="1"
                    column
                >
                    <v-chip
                        v-for="category in categories"
                        :key="category"
                        :value="category"
                        color="info"
                        variant="text"
                        class="font-weight-medium"
                        rounded-lg
                        filter
                    >
                        {{ category }}
                    </v-chip>
                </v-chip-group>
            </v-col>
        </v-row>

        <v-card class="pa-2 my-5" variant="flat">
            <v-row align="center">
                <v-col>
                    <v-text-field
                        v-model="search"
                        label="Search"
                        :prepend-inner-icon="Search"
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
                                :prepend-icon="ArrowUpDown"
                                :append-icon="ChevronDown"
                                v-bind="props"
                                class="mr-0 mr-sm-2 ml-n2"
                                title="Sort by"
                            >
                                <span class="text-body-2 hidden-xs">Sort by</span> <span class="ml-1 text-capitalize">{{ sortKey }}</span>
                            </v-btn>
                        </template>
                        <v-list class="pa-1">
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
                        <v-btn size="x-small" value="asc" title="Ascending" variant="text" rounded="xl">
                            <v-icon :icon="ArrowDownNarrowWide" />
                        </v-btn>
                        <v-btn size="x-small" value="desc" title="Descending" variant="text" rounded="xl">
                            <v-icon :icon="ArrowUpNarrowWide" />
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
                    <ApplicationItem v-if="showUplinkItem" :app="UplinkApp" />
                    <ApplicationItem v-for="app in items" :key="app.raw.name" :app="app.raw" />
                </v-row>
            </template>
        </v-data-iterator>
    </v-container>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import {
    VBtn,
    VBtnToggle,
    VCard,
    VChip,
    VChipGroup,
    VCol,
    VContainer,
    VDataIterator,
    VIcon,
    VList,
    VListItem,
    VMenu,
    VRow,
    VTextField,
} from 'vuetify/components';
import { ArrowDownNarrowWide, ArrowUpDown, ArrowUpNarrowWide, ChevronDown, Search } from 'lucide-vue-next';

import { AppCategory, Application, applications, UplinkApp } from '@/types/applications';
import { useTrialCheck } from '@/composables/useTrialCheck';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import ApplicationItem from '@/components/ApplicationItem.vue';
import TrialExpirationBanner from '@/components/TrialExpirationBanner.vue';

const { isTrialExpirationBanner, isUserProjectOwner, isExpired } = useTrialCheck();

const selectedChip = ref<AppCategory>(AppCategory.All);
const search = ref<string>('');
const sortKey = ref<string>('name');
const sortOrder = ref<string>('asc');
const sortKeys = ['Name', 'Category'];

/**
 * Indicates if uplink item should be shown.
 */
const showUplinkItem = computed<boolean>(() => !search.value && selectedChip.value === AppCategory.All);

/**
 * The sorting criteria to be used for the file list.
 */
const sortBy = computed(() => [{ key: sortKey.value, order: sortOrder.value }]);

/**
 * Returns all application categories.
 */
const categories = computed<string[]>(() => {
    const values = Object.values(AppCategory);
    values.shift();
    return values;
});

/**
 * Returns filtered apps based on selected category.
 */
const filteredApps = computed<Application[]>(() => {
    if (selectedChip.value === AppCategory.All) return applications;

    return applications.filter(app => selectedChip.value === app.category);
});
</script>

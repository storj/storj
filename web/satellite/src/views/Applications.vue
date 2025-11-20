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
                    class="border rounded-lg py-3 px-4 mt-4 bg-surface"
                    selected-class="font-weight-bold v-chip--variant-tonal"
                    mandatory
                    :max="1"
                >
                    <v-chip
                        v-for="category in categories"
                        :key="category"
                        :value="category"
                        variant="text"
                        class="font-weight-medium"
                        filter
                    >
                        {{ category }}
                    </v-chip>
                </v-chip-group>
            </v-col>
        </v-row>

        <v-card class="pa-4 mt-4 mb-4" variant="flat">
            <v-row align="center">
                <v-col cols="12" sm class="flex-grow-1 flex-sm-grow-1">
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
                        rounded="md"
                    />
                </v-col>
                <v-col cols="12" sm="auto" class="d-flex align-center">
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
                        variant="outlined"
                        color="default"
                        rounded="md"
                        class="pa-1"
                        mandatory
                    >
                        <v-btn value="asc" title="Ascending" variant="text" rounded="md">
                            <v-icon :icon="ArrowDownNarrowWide" />
                        </v-btn>
                        <v-btn value="desc" title="Descending" variant="text" rounded="md">
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
        >
            <template #no-data>
                <ApplicationItem />
            </template>
            <template #default="{ items }">
                <v-row>
                    <ApplicationItem v-for="app in items" :key="app.raw.name" :app="app.raw" />
                    <ApplicationItem />
                </v-row>
            </template>
        </v-data-iterator>
    </v-container>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref } from 'vue';
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
import { useRouter } from 'vue-router';

import { AppCategory, Application, applications, ObjectMountApp, UplinkApp } from '@/types/applications';
import { usePreCheck } from '@/composables/usePreCheck';
import { useConfigStore } from '@/store/modules/configStore';
import { ROUTES } from '@/router';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import ApplicationItem from '@/components/ApplicationItem.vue';
import TrialExpirationBanner from '@/components/TrialExpirationBanner.vue';

const { isTrialExpirationBanner, isUserProjectOwner, isExpired } = usePreCheck();
const router = useRouter();

const configStore = useConfigStore();

const selectedChip = ref<AppCategory>(AppCategory.All);
const search = ref<string>('');
const sortKey = ref<string>('name');
const sortOrder = ref<'asc' | 'desc'>('asc');
const sortKeys = ['Name', 'Category'];

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
    let result: Application[];
    if (selectedChip.value === AppCategory.All) {
        result = [...applications];
    } else {
        result = applications.filter(app => app.categories.includes(selectedChip.value));
    }

    result.sort((a, b) => {
        const aValue = (a[sortKey.value] || '').toLowerCase();
        const bValue = (b[sortKey.value] || '').toLowerCase();
        if (aValue < bValue) return sortOrder.value === 'asc' ? -1 : 1;
        if (aValue > bValue) return sortOrder.value === 'asc' ? 1 : -1;
        return 0;
    });

    const showAll = selectedChip.value === AppCategory.All && !search.value;
    if (showAll) {
        const index = result.findIndex(app => app.name === UplinkApp.name);
        if (index > -1) {
            const [uplink] = result.splice(index, 1);
            result.unshift(uplink);
        }
    }

    if (ObjectMountApp.categories.includes(selectedChip.value) || showAll) {
        const index = result.findIndex(app => app.name === ObjectMountApp.name);
        if (index > -1) {
            const [mount] = result.splice(index, 1);
            result.unshift(mount);
        }
    }

    return result;
});

onBeforeMount(() => {
    if (!configStore.isDefaultBrand) {
        router.replace({ name: ROUTES.Dashboard.name });
    }
});
</script>

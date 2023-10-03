// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-navigation-drawer v-model="model" class="py-1">
        <v-sheet>
            <v-list class="px-2" color="default" variant="flat">
                <template v-if="pathBeforeAccountPage">
                    <v-list-item class="pa-4 rounded-lg" link router-link :to="pathBeforeAccountPage" @click="() => registerLinkClick(pathBeforeAccountPage)">
                        <template #prepend>
                            <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <path d="M1 10C1 5.02944 5.02944 0.999999 10 0.999999C14.9706 0.999999 19 5.02944 19 10C19 14.9706 14.9706 19 10 19C5.02944 19 1 14.9706 1 10ZM1.99213 10C1.99213 14.4226 5.57737 18.0079 10 18.0079C14.4226 18.0079 18.0079 14.4226 18.0079 10C18.0079 5.57737 14.4226 1.99213 10 1.99213C5.57737 1.99213 1.99213 5.57737 1.99213 10ZM5.48501 9.73986L5.50374 9.7201L9.01144 6.2124C9.20516 6.01868 9.51925 6.01868 9.71297 6.2124C9.90024 6.39967 9.90648 6.69941 9.7317 6.89418L9.71297 6.91394L7.05211 9.5748L14.4646 9.5748C14.7385 9.5748 14.9606 9.7969 14.9606 10.0709C14.9606 10.3357 14.7531 10.5521 14.4918 10.5662L14.4646 10.5669L7.05211 10.5669L9.71297 13.2278C9.90024 13.4151 9.90648 13.7148 9.7317 13.9096L9.71297 13.9293C9.52571 14.1166 9.22597 14.1228 9.0312 13.9481L9.01144 13.9293L5.50374 10.4216C5.31647 10.2344 5.31023 9.93463 5.48501 9.73986Z" fill="currentColor" />
                            </svg>
                        </template>
                        <v-list-item-title link class="text-body-2 ml-3">
                            Go back
                        </v-list-item-title>
                    </v-list-item>

                    <v-divider class="my-2" />
                </template>

                <!-- All Projects -->
                <v-list-item class="pa-4 rounded-lg" link router-link to="/projects" @click="() => registerLinkClick('/projects')">
                    <template #prepend>
                        <icon-all-projects />
                    </template>
                    <v-list-item-title link class="text-body-2 ml-3">
                        All Projects
                    </v-list-item-title>
                </v-list-item>

                <v-list-item link router-link to="settings" class="my-1 py-3" rounded="lg" @click="() => registerLinkClick('/settings')">
                    <template #prepend>
                        <icon-settings />
                    </template>
                    <v-list-item-title class="text-body-2 ml-3">
                        Account Settings
                    </v-list-item-title>
                </v-list-item>

                <v-list-item link router-link to="billing" class="my-1" rounded="lg" @click="() => registerLinkClick('/billing')">
                    <template #prepend>
                        <icon-card />
                    </template>
                    <v-list-item-title class="text-body-2 ml-3">
                        Account Billing
                    </v-list-item-title>
                </v-list-item>

                <v-divider class="my-2" />
            </v-list>
        </v-sheet>
    </v-navigation-drawer>
</template>

<script setup lang="ts">
import { computed, onBeforeMount } from 'vue';
import {
    VNavigationDrawer,
    VSheet,
    VList,
    VListItem,
    VListItemTitle,
    VDivider,
} from 'vuetify/components';
import { useDisplay } from 'vuetify';

import { useAppStore } from '@poc/store/appStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import IconCard from '@poc/components/icons/IconCard.vue';
import IconSettings from '@poc/components/icons/IconSettings.vue';
import IconAllProjects from '@poc/components/icons/IconAllProjects.vue';

const analyticsStore = useAnalyticsStore();
const appStore = useAppStore();

const { mdAndDown } = useDisplay();

const model = computed<boolean>({
    get: () => appStore.state.isNavigationDrawerShown,
    set: value => appStore.toggleNavigationDrawer(value),
});

/**
 * Returns the path to the most recent non-account-related page.
 */
const pathBeforeAccountPage = computed((): string | null => {
    const path = appStore.state.pathBeforeAccountPage;
    if (!path || path === '/projects') return null;
    return path;
});

/**
 * Conditionally closes the navigation drawer and tracks page visit.
 */
function registerLinkClick(page: string | null): void {
    if (mdAndDown.value) {
        model.value = false;
    }
    trackPageVisitEvent(page);
}

/**
 * Sends "Page Visit" event to segment and opens link.
 */
function trackPageVisitEvent(page: string | null): void {
    if (page) analyticsStore.pageVisit(page);
}

onBeforeMount(() => {
    if (mdAndDown.value) {
        model.value = false;
    }
});
</script>

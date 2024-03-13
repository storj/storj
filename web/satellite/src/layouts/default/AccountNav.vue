// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-navigation-drawer v-model="model">
        <v-sheet>
            <v-list class="px-2 py-1" color="default" variant="flat">
                <!-- Back -->
                <template v-if="pathBeforeAccountPage">
                    <navigation-item class="pa-4" title="Back" :to="pathBeforeAccountPage">
                        <template #prepend>
                            <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <path d="M1 10C1 5.02944 5.02944 0.999999 10 0.999999C14.9706 0.999999 19 5.02944 19 10C19 14.9706 14.9706 19 10 19C5.02944 19 1 14.9706 1 10ZM1.99213 10C1.99213 14.4226 5.57737 18.0079 10 18.0079C14.4226 18.0079 18.0079 14.4226 18.0079 10C18.0079 5.57737 14.4226 1.99213 10 1.99213C5.57737 1.99213 1.99213 5.57737 1.99213 10ZM5.48501 9.73986L5.50374 9.7201L9.01144 6.2124C9.20516 6.01868 9.51925 6.01868 9.71297 6.2124C9.90024 6.39967 9.90648 6.69941 9.7317 6.89418L9.71297 6.91394L7.05211 9.5748L14.4646 9.5748C14.7385 9.5748 14.9606 9.7969 14.9606 10.0709C14.9606 10.3357 14.7531 10.5521 14.4918 10.5662L14.4646 10.5669L7.05211 10.5669L9.71297 13.2278C9.90024 13.4151 9.90648 13.7148 9.7317 13.9096L9.71297 13.9293C9.52571 14.1166 9.22597 14.1228 9.0312 13.9481L9.01144 13.9293L5.50374 10.4216C5.31647 10.2344 5.31023 9.93463 5.48501 9.73986Z" fill="currentColor" />
                            </svg>
                        </template>
                    </navigation-item>

                    <v-divider class="my-2" />
                </template>

                <!-- All Projects -->
                <navigation-item title="All Projects" subtitle="Dashboard" :to="ROUTES.Projects.path" class="py-4">
                    <template #prepend>
                        <icon-all-projects />
                    </template>
                </navigation-item>

                <v-divider class="my-2" />

                <v-list-item class="my-1">
                    <v-list-item-subtitle>My Account</v-list-item-subtitle>
                </v-list-item>

                <v-list-item v-if="!isPaidTier && billingEnabled" link lines="one" class="my-1 py-2" tabindex="0" @click="toggleUpgradeFlow" @keydown.space.prevent="toggleUpgradeFlow">
                    <template #prepend>
                        <icon-upgrade size="20" />
                    </template>
                    <v-list-item-title class="ml-3">Upgrade</v-list-item-title>
                </v-list-item>

                <!-- Account Billing -->
                <navigation-item v-if="billingEnabled" title="Billing" :to="ROUTES.Account.with(ROUTES.Billing).path" class="py-2">
                    <template #prepend>
                        <icon-card />
                    </template>
                </navigation-item>

                <!-- Account Settings -->
                <navigation-item title="Settings" :to="ROUTES.Account.with(ROUTES.AccountSettings).path" class="py-2">
                    <template #prepend>
                        <icon-settings />
                    </template>
                </navigation-item>

                <v-divider class="my-2" />

                <v-menu location="end" transition="scale-transition">
                    <template #activator="{ props: activatorProps }">
                        <navigation-item title="Resources" v-bind="activatorProps">
                            <template #prepend>
                                <IconResources />
                            </template>
                            <template #append>
                                <img src="@/assets/icon-right.svg" alt="Resources" width="10">
                            </template>
                        </navigation-item>
                    </template>

                    <v-list class="pa-2">
                        <v-list-item
                            class="py-3"
                            href="https://docs.storj.io/"
                            target="_blank"
                            rel="noopener noreferrer"
                            @click="() => trackViewDocsEvent('https://docs.storj.io/')"
                        >
                            <template #prepend>
                                <IconDocs />
                            </template>
                            <v-list-item-title class="mx-3">
                                Documentation
                            </v-list-item-title>
                            <v-list-item-subtitle class="mx-3">
                                <small>Go to the Storj docs.</small>
                            </v-list-item-subtitle>
                        </v-list-item>

                        <v-list-item
                            class="py-3"
                            href="https://forum.storj.io/"
                            target="_blank"
                            rel="noopener noreferrer"
                            @click="() => trackViewForumEvent('https://forum.storj.io/')"
                        >
                            <template #prepend>
                                <IconForum />
                            </template>
                            <v-list-item-title class="mx-3">
                                Community Forum
                            </v-list-item-title>
                            <v-list-item-subtitle class="mx-3">
                                <small>Join our global community.</small>
                            </v-list-item-subtitle>
                        </v-list-item>

                        <v-list-item
                            class="py-3"
                            href="https://supportdcs.storj.io/hc/en-us"
                            target="_blank"
                            rel="noopener noreferrer"
                            @click="() => trackViewSupportEvent('https://supportdcs.storj.io/hc/en-us')"
                        >
                            <template #prepend>
                                <IconSupport />
                            </template>
                            <v-list-item-title class="mx-3">
                                Storj Support
                            </v-list-item-title>
                            <v-list-item-subtitle class="mx-3">
                                <small>Need help? Get support.</small>
                            </v-list-item-subtitle>
                        </v-list-item>
                    </v-list>
                </v-menu>

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
    VListItemSubtitle,
    VDivider,
    VMenu,
} from 'vuetify/components';
import { useDisplay } from 'vuetify';

import { useAppStore } from '@/store/modules/appStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useUsersStore } from '@/store/modules/usersStore.js';
import { ROUTES } from '@/router';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames.js';

import IconCard from '@/components/icons/IconCard.vue';
import IconSettings from '@/components/icons/IconSettings.vue';
import NavigationItem from '@/layouts/default/NavigationItem.vue';
import IconAllProjects from '@/components/icons/IconAllProjects.vue';
import IconDocs from '@/components/icons/IconDocs.vue';
import IconForum from '@/components/icons/IconForum.vue';
import IconSupport from '@/components/icons/IconSupport.vue';
import IconResources from '@/components/icons/IconResources.vue';
import IconUpgrade from '@/components/icons/IconUpgrade.vue';

const analyticsStore = useAnalyticsStore();
const appStore = useAppStore();
const configStore = useConfigStore();
const usersStore = useUsersStore();

const { mdAndDown } = useDisplay();

const model = computed<boolean>({
    get: () => appStore.state.isNavigationDrawerShown,
    set: value => appStore.toggleNavigationDrawer(value),
});

/**
 * Returns user's paid tier status from store.
 */
const isPaidTier = computed<boolean>(() => usersStore.state.user.paidTier ?? false);

/**
 * Indicates if billing features are enabled.
 */
const billingEnabled = computed<boolean>(() => configStore.getBillingEnabled(usersStore.state.user.hasVarPartner));

/**
 * Returns the path to the most recent non-account-related page.
 */
const pathBeforeAccountPage = computed((): string | null => {
    const path = appStore.state.pathBeforeAccountPage;
    if (!path || path === ROUTES.Projects.path) return null;
    return path;
});

/**
 * Toggles upgrade account flow visibility.
 */
function toggleUpgradeFlow(): void {
    if (mdAndDown.value) {
        model.value = false;
    }
    appStore.toggleUpgradeFlow(true);
}

/**
 * Sends "View Docs" event to segment.
 */
function trackViewDocsEvent(link: string): void {
    analyticsStore.pageVisit(link);
    analyticsStore.eventTriggered(AnalyticsEvent.VIEW_DOCS_CLICKED);
}

/**
 * Sends "View Forum" event to segment.
 */
function trackViewForumEvent(link: string): void {
    analyticsStore.pageVisit(link);
    analyticsStore.eventTriggered(AnalyticsEvent.VIEW_FORUM_CLICKED);
}

/**
 * Sends "View Support" event to segment.
 */
function trackViewSupportEvent(link: string): void {
    analyticsStore.pageVisit(link);
    analyticsStore.eventTriggered(AnalyticsEvent.VIEW_SUPPORT_CLICKED);
}

onBeforeMount(() => {
    if (mdAndDown.value) {
        model.value = false;
    }
});
</script>

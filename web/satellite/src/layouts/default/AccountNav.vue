// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-navigation-drawer v-model="model">
        <v-sheet>
            <v-list class="px-2 py-1" variant="flat" active-class="text-primary">
                <!-- Back -->
                <template v-if="pathBeforeAccountPage">
                    <navigation-item class="pa-4" title="Back" :to="pathBeforeAccountPage">
                        <template #prepend>
                            <component :is="ArrowLeft" :size="18" />
                        </template>
                    </navigation-item>

                    <v-divider class="my-2" />
                </template>

                <!-- All Projects -->
                <navigation-item title="All Projects" subtitle="Dashboard" :to="ROUTES.Projects.path" class="py-4" @click="closeDrawer">
                    <template #prepend>
                        <component :is="Layers" :size="18" />
                    </template>
                </navigation-item>

                <v-divider class="my-2" />

                <v-list-item class="my-1">
                    <v-list-item-subtitle>My Account</v-list-item-subtitle>
                </v-list-item>

                <v-list-item v-if="!hasPaidPrivileges && billingEnabled" link lines="one" class="my-1 py-2" tabindex="0" @click="toggleUpgradeFlow" @keydown.space.prevent="toggleUpgradeFlow">
                    <template #prepend>
                        <component :is="CircleArrowUp" :size="18" />
                    </template>
                    <v-list-item-title class="ml-4">Upgrade</v-list-item-title>
                </v-list-item>

                <!-- Account Billing -->
                <navigation-item v-if="billingEnabled" title="Billing" :to="ROUTES.Account.with(ROUTES.Billing).path" class="py-2">
                    <template #prepend>
                        <component :is="CreditCard" :size="18" />
                    </template>
                </navigation-item>

                <navigation-item v-if="hasPaidPrivileges && apiKeysUiEnabled" title="API Keys" :to="ROUTES.Account.with(ROUTES.APIKeys).path" class="py-2">
                    <template #prepend>
                        <component :is="Key" :size="18" />
                    </template>
                </navigation-item>

                <!-- Account Settings -->
                <navigation-item title="Settings" :to="ROUTES.Account.with(ROUTES.AccountSettings).path" class="py-2">
                    <template #prepend>
                        <component :is="Settings2" :size="18" />
                    </template>
                </navigation-item>

                <v-divider class="my-2" />

                <v-menu location="end" transition="scale-transition">
                    <template #activator="{ props: activatorProps }">
                        <navigation-item title="Resources" v-bind="activatorProps">
                            <template #prepend>
                                <component :is="BookMarked" :size="18" />
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
                                <component :is="BookOpenText" :size="18" />
                            </template>
                            <v-list-item-title class="mx-4">
                                Documentation
                            </v-list-item-title>
                            <v-list-item-subtitle class="mx-4">
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
                                <component :is="MessagesSquare" :size="18" />
                            </template>
                            <v-list-item-title class="mx-4">
                                Community Forum
                            </v-list-item-title>
                            <v-list-item-subtitle class="mx-4">
                                <small>Join our global community.</small>
                            </v-list-item-subtitle>
                        </v-list-item>

                        <v-list-item
                            class="py-3"
                            :href="configStore.supportUrl"
                            target="_blank"
                            rel="noopener noreferrer"
                            @click="() => trackViewSupportEvent(configStore.supportUrl)"
                        >
                            <template #prepend>
                                <component :is="MessageCircleQuestion" :size="18" />
                            </template>
                            <v-list-item-title class="mx-4">
                                Storj Support
                            </v-list-item-title>
                            <v-list-item-subtitle class="mx-4">
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
import {
    BookMarked,
    Layers,
    Settings2,
    CreditCard,
    CircleArrowUp,
    ArrowLeft,
    MessagesSquare,
    MessageCircleQuestion,
    BookOpenText,
    Key,
} from 'lucide-vue-next';

import { useAppStore } from '@/store/modules/appStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useUsersStore } from '@/store/modules/usersStore.js';
import { ROUTES } from '@/router';
import { AnalyticsEvent, PageVisitSource } from '@/utils/constants/analyticsEventNames.js';

import NavigationItem from '@/layouts/default/NavigationItem.vue';

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
const hasPaidPrivileges = computed<boolean>(() => usersStore.state.user.hasPaidPrivileges);

/**
 * Indicates if billing features are enabled.
 */
const billingEnabled = computed<boolean>(() => configStore.getBillingEnabled(usersStore.state.user));

/**
 * Indicates if REST API keys UI are enabled.
 */
const apiKeysUiEnabled = computed<boolean>(() => configStore.state.config.restAPIKeysUIEnabled);

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
    closeDrawer();
    appStore.toggleUpgradeFlow(true);
}

/**
 * Conditionally closes the navigation drawer.
 */
function closeDrawer(): void {
    if (mdAndDown.value) {
        model.value = false;
    }
}

/**
 * Sends "View Docs" event to segment.
 */
function trackViewDocsEvent(link: string): void {
    closeDrawer();
    analyticsStore.pageVisit(link, PageVisitSource.DOCS);
    analyticsStore.eventTriggered(AnalyticsEvent.VIEW_DOCS_CLICKED);
}

/**
 * Sends "View Forum" event to segment.
 */
function trackViewForumEvent(link: string): void {
    closeDrawer();
    analyticsStore.pageVisit(link, PageVisitSource.FORUM);
    analyticsStore.eventTriggered(AnalyticsEvent.VIEW_FORUM_CLICKED);
}

/**
 * Sends "View Support" event to segment.
 */
function trackViewSupportEvent(link: string): void {
    closeDrawer();
    analyticsStore.pageVisit(link, PageVisitSource.SUPPORT);
    analyticsStore.eventTriggered(AnalyticsEvent.VIEW_SUPPORT_CLICKED);
}

onBeforeMount(() => {
    closeDrawer();
});
</script>

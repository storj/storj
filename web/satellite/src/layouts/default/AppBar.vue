// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app-bar :elevation="0" class="app-bar-border">
        <v-progress-linear indeterminate absolute location="bottom" color="primary" :active="appStore.state.isNavigating" height="3" />

        <v-app-bar-nav-icon
            v-if="showNavDrawerButton"
            variant="text"
            color="default"
            class="ml-3 ml-sm-5 mr-0 mr-sm-1"
            size="small"
            density="comfortable"
            title="Toggle sidebar navigation"
            @click.stop="appStore.toggleNavigationDrawer()"
        />

        <v-app-bar-title class="mt-n1 ml-1 mr-2 flex-initial" :class="{ 'ml-4': !showNavDrawerButton }">
            <router-link :to="ROUTES.Projects.path">
                <v-img
                    v-if="theme.global.current.value.dark"
                    src="@/assets/logo-dark.svg"
                    width="120"
                    alt="Storj Logo"
                />
                <v-img
                    v-else
                    src="@/assets/logo.svg"
                    width="120"
                    alt="Storj Logo"
                />
            </router-link>
        </v-app-bar-title>

        <template #append>
            <v-menu offset-y width="200" class="rounded-xl">
                <template #activator="{ props: activatorProps }">
                    <!-- Theme Toggle Light/Dark Mode -->
                    <v-btn-toggle
                        v-model="activeTheme"
                        mandatory
                        border
                        inset
                        density="comfortable"
                        class="pa-1"
                    >
                        <v-tooltip text="Light Theme" location="bottom">
                            <template #activator="{ props: darkProps }">
                                <v-btn
                                    v-bind="darkProps"
                                    rounded="xl"
                                    :icon="Sun"
                                    size="small"
                                    class="px-4"
                                    aria-label="Toggle Light Theme"
                                    @click="toggleTheme('light')"
                                />
                            </template>
                        </v-tooltip>

                        <v-tooltip text="Dark Theme" location="bottom">
                            <template #activator="{ props: lightProps }">
                                <v-btn
                                    v-bind="lightProps"
                                    rounded="xl"
                                    :icon="MoonStar"
                                    size="small"
                                    class="px-4"
                                    aria-label="Toggle Dark Theme"
                                    @click="toggleTheme('dark')"
                                />
                            </template>
                        </v-tooltip>
                    </v-btn-toggle>

                    <!-- My Account Dropdown Button -->
                    <v-btn
                        v-bind="activatorProps"
                        variant="outlined"
                        color="default"
                        class="ml-2 ml-sm-3 font-weight-medium"
                    >
                        <template #append>
                            <img src="@/assets/icon-dropdown.svg" alt="Account Dropdown">
                        </template>
                        My Account
                    </v-btn>
                </template>

                <!-- My Account Menu -->
                <v-list class="px-2 rounded-lg">
                    <v-list-item class="py-2">
                        <v-list-item-title class="text-body-2">
                            Account
                        </v-list-item-title>
                        <v-list-item-subtitle>
                            {{ user.email }}
                            <v-tooltip activator="parent" location="top">
                                {{ user.email }}
                            </v-tooltip>
                        </v-list-item-subtitle>
                    </v-list-item>

                    <v-list-item v-if="billingEnabled" class="py-2">
                        <v-list-item-title class="text-body-2">
                            <v-chip
                                class="font-weight-bold"
                                :color="isPaidTier ? 'success' : 'info'"
                                variant="tonal"
                                size="small"
                            >
                                {{ isPaidTier ? 'Pro Account' : 'Free Trial' }}
                            </v-chip>
                        </v-list-item-title>
                    </v-list-item>

                    <v-list-item class="py-2 text-medium-emphasis">
                        <template #prepend>
                            <icon-satellite size="18" />
                            <v-tooltip activator="parent" location="top">
                                Satellite (Metadata Region) <a href="https://docs.storj.io/learn/concepts/satellite" target="_blank" class="link">Learn More</a>
                            </v-tooltip>
                        </template>
                        <v-list-item-title class="text-body-2 ml-4">
                            {{ satelliteName }}
                        </v-list-item-title>
                    </v-list-item>

                    <template v-if="billingEnabled">
                        <v-list-item v-if="!isPaidTier" link class="my-1" @click="toggleUpgradeFlow">
                            <template #prepend>
                                <icon-upgrade size="18" />
                            </template>
                            <v-list-item-title class="text-body-2 ml-4">
                                Upgrade
                            </v-list-item-title>
                        </v-list-item>
                    </template>

                    <v-list-item v-if="billingEnabled" link class="my-1" router-link :to="billingPath" @click="closeSideNav">
                        <template #prepend>
                            <icon-card size="18" />
                        </template>
                        <v-list-item-title class="text-body-2 ml-4">
                            Billing
                        </v-list-item-title>
                    </v-list-item>

                    <v-list-item link class="my-1" router-link :to="settingsPath" @click="closeSideNav">
                        <template #prepend>
                            <icon-settings size="18" />
                        </template>
                        <v-list-item-title class="text-body-2 ml-4">
                            Settings
                        </v-list-item-title>
                    </v-list-item>
                    <v-list-item link @click="onLogout">
                        <template #prepend>
                            <icon-logout size="18" />
                        </template>
                        <v-list-item-title class="text-body-2 ml-4">
                            Sign Out
                        </v-list-item-title>
                    </v-list-item>
                </v-list>
            </v-menu>
        </template>
    </v-app-bar>

    <account-setup-dialog />
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue';
import { useDisplay, useTheme } from 'vuetify';
import {
    VAppBar,
    VAppBarNavIcon,
    VAppBarTitle,
    VImg,
    VMenu,
    VBtnToggle,
    VTooltip,
    VBtn,
    VList,
    VListItem,
    VListItemTitle,
    VListItemSubtitle,
    VChip,
    VProgressLinear,
} from 'vuetify/components';
import { MoonStar, Sun } from 'lucide-vue-next';

import { useAppStore } from '@/store/modules/appStore';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';
import { ROUTES } from '@/router';
import { User } from '@/types/users';
import { useLogout } from '@/composables/useLogout';

import IconCard from '@/components/icons/IconCard.vue';
import IconUpgrade from '@/components/icons/IconUpgrade.vue';
import IconSettings from '@/components/icons/IconSettings.vue';
import IconLogout from '@/components/icons/IconLogout.vue';
import IconSatellite from '@/components/icons/IconSatellite.vue';
import AccountSetupDialog from '@/components/dialogs/AccountSetupDialog.vue';

const appStore = useAppStore();
const usersStore = useUsersStore();
const configStore = useConfigStore();

const theme = useTheme();
const notify = useNotify();
const { mdAndDown } = useDisplay();
const { logout } = useLogout();

const settingsPath = ROUTES.Account.with(ROUTES.AccountSettings).path;
const billingPath = ROUTES.Account.with(ROUTES.Billing).path;

const activeTheme = ref<number>(0);

withDefaults(defineProps<{
    showNavDrawerButton: boolean;
}>(), {
    showNavDrawerButton: false,
});

/**
 * Indicates if billing features are enabled.
 */
const billingEnabled = computed<boolean>(() => configStore.getBillingEnabled(usersStore.state.user.hasVarPartner));

/**
 * Returns the name of the current satellite.
 */
const satelliteName = computed<string>(() => {
    return configStore.state.config.satelliteName;
});

/**
 * Returns user entity from store.
 */
const user = computed<User>(() => usersStore.state.user);

/**
 * Indicates if user is in paid tier.
 */
const isPaidTier = computed<boolean>(() => {
    return user.value.paidTier ?? false;
});

function toggleTheme(newTheme: string): void {
    if ((newTheme === 'dark' && theme.global.current.value.dark) || (newTheme === 'light' && !theme.global.current.value.dark)) {
        return;
    }
    theme.global.name.value = newTheme;
    localStorage.setItem('theme', newTheme);  // Store the selected theme in localStorage
}

watch(() => theme.global.current.value.dark, (newVal: boolean) => {
    activeTheme.value = newVal ? 1 : 0;
}, { immediate: true });

function closeSideNav(): void {
    if (mdAndDown.value) appStore.toggleNavigationDrawer(false);
}

function toggleUpgradeFlow(): void {
    closeSideNav();
    appStore.toggleUpgradeFlow(true);
}

/**
 * Logs out user and navigates to login page.
 */
async function onLogout(): Promise<void> {
    try {
        await logout();
    } catch (error) {
        notify.error(error.message);
    }
}

</script>

<style scoped lang="scss">
.flex-initial {
    flex: initial;
}

:deep(.v-chip__content) {
    display: inline-block !important;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
</style>

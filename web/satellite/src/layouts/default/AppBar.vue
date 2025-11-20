// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app-bar class="app-bar-border">
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
                    v-if="themeStore.globalTheme?.dark"
                    :src="configStore.darkLogo"
                    width="120"
                    alt="Logo"
                />
                <v-img
                    v-else
                    :src="configStore.logo"
                    width="120"
                    alt="Logo"
                />
            </router-link>
        </v-app-bar-title>

        <template #append>
            <v-menu offset-y width="200" class="rounded-xl">
                <template #activator="{ props: activatorProps }">
                    <v-btn
                        v-bind="activatorProps"
                        variant="outlined"
                        color="default"
                        rounded="lg"
                        :icon="activeThemeIcon"
                    />
                </template>

                <v-list class="px-2 rounded-lg">
                    <v-list-item :active="activeTheme === 0" class="px-2" @click="themeStore.setTheme('light')">
                        <v-list-item-title class="text-body-2">
                            <v-btn
                                class="mr-2"
                                variant="outlined"
                                color="default"
                                size="x-small"
                                rounded="lg"
                                :icon="Sun"
                            />
                            Light
                        </v-list-item-title>
                    </v-list-item>

                    <v-list-item :active="activeTheme === 1" class="px-2" @click="themeStore.setTheme('dark')">
                        <v-list-item-title class="text-body-2">
                            <v-btn
                                class="mr-2"
                                variant="outlined"
                                color="default"
                                size="x-small"
                                rounded="lg"
                                :icon="MoonStar"
                            />
                            Dark
                        </v-list-item-title>
                    </v-list-item>

                    <v-list-item :active="activeTheme === 2" class="px-2" @click="themeStore.setTheme('auto')">
                        <v-list-item-title class="text-body-2">
                            <v-btn
                                class="mr-2"
                                variant="outlined"
                                color="default"
                                size="x-small"
                                rounded="lg"
                                :icon="smAndDown ? Smartphone : Monitor"
                            />
                            System
                        </v-list-item-title>
                    </v-list-item>
                </v-list>
            </v-menu>
            <v-menu offset-y width="200" class="rounded-xl">
                <template #activator="{ props: activatorProps }">
                    <!-- My Account Dropdown Button -->
                    <v-btn
                        v-bind="activatorProps"
                        variant="outlined"
                        color="default"
                        class="ml-2 ml-sm-3 mr-sm-2 font-weight-medium"
                    >
                        <template #append>
                            <img src="@/assets/icon-dropdown.svg" alt="Account Dropdown">
                        </template>
                        My Account
                    </v-btn>
                </template>

                <!-- My Account Menu -->
                <v-list class="px-2 rounded-lg" active-class="text-primary">
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

                    <v-list-item v-if="billingEnabled || user.isNFR" class="py-2">
                        <v-list-item-title class="text-body-2">
                            <v-chip
                                class="font-weight-bold"
                                :color="isPaidTier ? 'success' : user.isNFR ? 'warning' : 'info'"
                                variant="tonal"
                                size="small"
                            >
                                {{ user.kind.name }}
                            </v-chip>
                        </v-list-item-title>
                    </v-list-item>

                    <v-list-item class="py-2 text-medium-emphasis">
                        <template #prepend>
                            <icon-satellite size="18" />
                            <v-tooltip activator="parent" location="top">
                                Satellite (Metadata Region) <a href="https://docs.storj.io/learn/concepts/satellite" target="_blank" class="link" rel="noopener noreferrer">Learn More</a>
                            </v-tooltip>
                        </template>
                        <v-list-item-title class="text-body-2 ml-4">
                            {{ satelliteName }}
                        </v-list-item-title>
                    </v-list-item>

                    <template v-if="billingEnabled">
                        <v-list-item v-if="!isPaidTier" link class="my-1" @click="toggleUpgradeFlow">
                            <template #prepend>
                                <component :is="CircleArrowUp" :size="18" />
                            </template>
                            <v-list-item-title class="text-body-2 ml-4">
                                Upgrade
                            </v-list-item-title>
                        </v-list-item>
                    </template>

                    <v-list-item v-if="billingEnabled" link class="my-1" router-link :to="billingPath" @click="closeSideNav">
                        <template #prepend>
                            <component :is="CreditCard" :size="18" />
                        </template>
                        <v-list-item-title class="text-body-2 ml-4">
                            Billing
                        </v-list-item-title>
                    </v-list-item>

                    <v-list-item link class="my-1" router-link :to="settingsPath" @click="closeSideNav">
                        <template #prepend>
                            <component :is="Settings2" :size="18" />
                        </template>
                        <v-list-item-title class="text-body-2 ml-4">
                            Settings
                        </v-list-item-title>
                    </v-list-item>
                    <v-list-item v-if="userFeedbackEnabled" link class="my-1" @click="toggleUserFeedback">
                        <template #prepend>
                            <component :is="MessageCircle" :size="18" />
                        </template>
                        <v-list-item-title class="text-body-2 ml-4">
                            Give Feedback
                        </v-list-item-title>
                    </v-list-item>
                    <v-list-item link @click="onLogout">
                        <template #prepend>
                            <component :is="LogOut" :size="18" />
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
    <user-feedback-dialog v-if="userFeedbackEnabled" v-model="userFeedbackDialogShown" />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useDisplay } from 'vuetify';
import {
    VAppBar,
    VAppBarNavIcon,
    VAppBarTitle,
    VBtn,
    VChip,
    VImg,
    VList,
    VListItem,
    VListItemSubtitle,
    VListItemTitle,
    VMenu,
    VProgressLinear,
    VTooltip,
} from 'vuetify/components';
import { CircleArrowUp, CreditCard, LogOut, Monitor, MoonStar, Settings2, Smartphone, Sun, MessageCircle } from 'lucide-vue-next';

import { useAppStore } from '@/store/modules/appStore';
import { useNotify } from '@/composables/useNotify';
import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';
import { ROUTES } from '@/router';
import { User } from '@/types/users';
import { useLogout } from '@/composables/useLogout';
import { useThemeStore } from '@/store/modules/themeStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import IconSatellite from '@/components/icons/IconSatellite.vue';
import AccountSetupDialog from '@/components/dialogs/AccountSetupDialog.vue';
import UserFeedbackDialog from '@/components/dialogs/UserFeedbackDialog.vue';

const appStore = useAppStore();
const usersStore = useUsersStore();
const configStore = useConfigStore();

const themeStore = useThemeStore();
const notify = useNotify();
const { mdAndDown, smAndDown } = useDisplay();
const { logout } = useLogout();

const settingsPath = ROUTES.Account.with(ROUTES.AccountSettings).path;
const billingPath = ROUTES.Account.with(ROUTES.Billing).path;

withDefaults(defineProps<{
    showNavDrawerButton?: boolean;
}>(), {
    showNavDrawerButton: false,
});

const userFeedbackDialogShown = ref<boolean>(false);

const userFeedbackEnabled = computed<boolean>(() => configStore.state.config.userFeedbackEnabled);

const activeTheme = computed<number>(() => {
    switch (themeStore.state.name) {
    case 'light':
        return 0;
    case 'dark':
        return 1;
    default:
        return 2;
    }
});

const activeThemeIcon = computed(() => {
    switch (themeStore.state.name) {
    case 'light':
        return Sun;
    case 'dark':
        return MoonStar;
    default:
        return themeStore.globalTheme?.dark ? MoonStar : Sun;
    }
});

/**
 * Indicates if billing features are enabled.
 */
const billingEnabled = computed<boolean>(() => configStore.getBillingEnabled(usersStore.state.user));

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
const isPaidTier = computed<boolean>(() => user.value.isPaid);

function closeSideNav(): void {
    if (mdAndDown.value) appStore.toggleNavigationDrawer(false);
}

function toggleUpgradeFlow(): void {
    closeSideNav();
    appStore.toggleUpgradeFlow(true);
}

function toggleUserFeedback(): void {
    closeSideNav();
    userFeedbackDialogShown.value = true;
}

/**
 * Logs out user and navigates to login page.
 */
async function onLogout(): Promise<void> {
    try {
        await logout();
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.APPLICATION_BAR);
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
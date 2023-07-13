// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app-bar :elevation="0">
        <v-app-bar-nav-icon
            variant="text"
            color="default"
            class="ml-1"
            size="x-small"
            density="comfortable"
            @click.stop="drawer = !drawer"
        />

        <v-app-bar-title class="mx-1">
            <v-img
                v-if="theme.global.current.value.dark"
                src="@poc/assets/logo-dark.svg"
                width="120"
                alt="Storj Logo"
            />
            <v-img
                v-else
                src="@poc/assets/logo.svg"
                width="120"
                alt="Storj Logo"
            />
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
                                    icon="mdi-weather-sunny"
                                    size="small"
                                    rounded="xl"
                                    aria-label="Toggle Light Theme"
                                    @click="toggleTheme('light')"
                                />
                            </template>
                        </v-tooltip>

                        <v-tooltip text="Dark Theme" location="bottom">
                            <template #activator="{ props: lightProps }">
                                <v-btn
                                    v-bind="lightProps"
                                    icon="mdi-weather-night"
                                    size="small"
                                    rounded="xl"
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
                        class="ml-4 font-weight-medium"
                        density="comfortable"
                    >
                        <template #append>
                            <img src="@poc/assets/icon-dropdown.svg" alt="Account Dropdown">
                        </template>
                        My Account
                    </v-btn>
                </template>

                <!-- My Account Menu -->
                <v-list class="px-2">
                    <v-list-item class="py-2 rounded-lg">
                        <template #prepend>
                            <img src="@poc/assets/icon-satellite.svg" alt="Region">
                        </template>
                        <v-list-item-title class="text-body-2 ml-3">Region</v-list-item-title>
                        <v-list-item-subtitle class="ml-3">
                            North America 1
                        </v-list-item-subtitle>
                    </v-list-item>

                    <v-divider class="my-2" />

                    <v-list-item link class="my-1 rounded-lg">
                        <template #prepend>
                            <img src="@poc/assets/icon-upgrade.svg" alt="Upgrade">
                        </template>
                        <v-list-item-title class="text-body-2 ml-3">
                            Upgrade
                        </v-list-item-title>
                    </v-list-item>

                    <v-list-item link class="my-1 rounded-lg" router-link to="/billing">
                        <template #prepend>
                            <img src="@poc/assets/icon-card.svg" alt="Billing">
                        </template>
                        <v-list-item-title class="text-body-2 ml-3">
                            Billing
                        </v-list-item-title>
                    </v-list-item>

                    <v-list-item link class="my-1 rounded-lg" router-link to="/account-settings">
                        <template #prepend>
                            <img src="@poc/assets/icon-settings.svg" alt="Account Settings">
                        </template>
                        <v-list-item-title class="text-body-2 ml-3">
                            Settings
                        </v-list-item-title>
                    </v-list-item>
                    <v-list-item class="rounded-lg" link>
                        <template #prepend>
                            <img src="@poc/assets/icon-logout.svg" alt="Log Out">
                        </template>
                        <v-list-item-title class="text-body-2 ml-3">
                            Sign Out
                        </v-list-item-title>
                    </v-list-item>
                </v-list>
            </v-menu>
        </template>
    </v-app-bar>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import { useTheme } from 'vuetify';
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
    VDivider,
} from 'vuetify/components';

const drawer = ref<boolean>(true);
const activeTheme = ref<number>(0);

const theme = useTheme();

function toggleTheme(newTheme: string): void {
    if ((newTheme === 'dark' && theme.global.current.value.dark) || (newTheme === 'light' && !theme.global.current.value.dark)) {
        return;
    }
    theme.global.name.value = newTheme;
    localStorage.setItem('theme', newTheme);  // Store the selected theme in localStorage
}

watch(() => theme.global.current.value.dark, (newVal: boolean) => {
    activeTheme.value = newVal ? 1 : 0;
});

// Check for stored theme in localStorage. If none, default to 'light'
toggleTheme(localStorage.getItem('theme') || 'light');
activeTheme.value = theme.global.current.value.dark ? 1 : 0;
</script>

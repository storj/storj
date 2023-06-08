// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app-bar :elevation="0" :border="true">
        <v-app-bar-nav-icon variant="text" color="light" class="ml-2" size="x-small" density="comfortable" @click.stop="drawer = !drawer" />

        <v-app-bar-title class="ml-1">
            <v-img v-if="theme.global.current.value.dark" :src="LogoWhite" width="180" alt="Storj Logo" />
            <v-img v-else :src="Logo" width="180" alt="Storj Logo" />
        </v-app-bar-title>

        <template #append>
            <v-menu offset-y width="200">
                <template #activator="{ props }">
                    <!-- Theme Toggle Light/Dark Mode -->
                    <v-btn-toggle v-model="activeTheme" mandatory>
                        <v-btn icon="mdi-weather-sunny" size="small" rounded="xl" @click="() => toggleTheme('light')" />
                        <v-btn icon="mdi-weather-night" size="small" rounded="xl" @click="() => toggleTheme('dark')" />
                    </v-btn-toggle>

                    <!-- My Account Dropdown Button -->
                    <v-btn v-bind="props" variant="text" color="light" class="ml-4 mr-1 font-weight-medium">
                        <template #prepend>
                            <img :src="AccountIcon" alt="Account">
                        </template>
                        My Account
                    </v-btn>
                </template>

                <!-- My Account Menu -->
                <v-list>
                    <v-list-item class="pt-2 pb-4 border-b">
                        <template #prepend>
                            <img :src="SatelliteIcon" alt="Region" class="mr-3">
                        </template>
                        <v-list-item-title class="text-body-2">Region</v-list-item-title>
                        <v-list-item-subtitle>
                            North America 1
                        </v-list-item-subtitle>
                    </v-list-item>
                    <v-list-item link>
                        <template #prepend>
                            <img :src="UpgradeIcon" alt="Upgrade" class="mr-3">
                        </template>
                        <v-list-item-title class="text-body-2">
                            Upgrade
                        </v-list-item-title>
                    </v-list-item>
                    <v-list-item link router-link to="/account-settings">
                        <template #prepend>
                            <img :src="SettingsIcon" alt="Account Settings" class="mr-3">
                        </template>
                        <v-list-item-title class="text-body-2">
                            Settings
                        </v-list-item-title>
                    </v-list-item>
                    <v-list-item link router-link to="/billing">
                        <template #prepend>
                            <img :src="CardIcon" alt="Billing" class="mr-3">
                        </template>
                        <v-list-item-title class="text-body-2">
                            Billing
                        </v-list-item-title>
                    </v-list-item>
                    <v-list-item link>
                        <template #prepend>
                            <img :src="LogoutIcon" alt="Log Out" class="mr-3">
                        </template>
                        <v-list-item-title class="text-body-2">
                            Sign Out
                        </v-list-item-title>
                    </v-list-item>
                </v-list>
            </v-menu>
        </template>
    </v-app-bar>

    <v-navigation-drawer
        v-model="drawer"
        class="py-1"
    >
        <v-sheet>
            <v-list>
                <v-list-item link class="py-2">
                    <v-menu activator="parent" location="end" transition="scale-transition">
                        <v-card min-width="300">
                            <v-list>
                                <v-list-item link>
                                    <template #prepend>
                                        <img :src="ProjectsIcon" alt="All Projects" class="mr-3">
                                    </template>
                                    <v-list-item-title class="text-body-2">
                                        View all projects
                                    </v-list-item-title>
                                </v-list-item>
                                <v-divider />
                                <v-list-item link>
                                    <template #prepend>
                                        <img :src="PlusIcon" alt="New Project" class="mr-3">
                                    </template>
                                    <v-list-item-title class="text-body-2">
                                        Create new project
                                    </v-list-item-title>
                                </v-list-item>
                                <v-divider />
                                <v-list-item link>
                                    <template #prepend>
                                        <img :src="LockIcon" alt="Passphrase" class="mr-3">
                                    </template>
                                    <v-list-item-title class="text-body-2">
                                        Manage Passphrase
                                    </v-list-item-title>
                                </v-list-item>
                            </v-list>
                        </v-card>
                    </v-menu>
                    <template #prepend>
                        <img :src="ProjectIcon" alt="Project" class="mr-3">
                    </template>
                    <v-list-item-title link class="text-body-2">
                        Project
                    </v-list-item-title>
                    <v-list-item-subtitle>
                        My first project
                    </v-list-item-subtitle>
                    <template #append>
                        <img :src="RightIcon" alt="Project" width="10">
                    </template>
                </v-list-item>

                <v-divider class="my-2" />

                <v-list-item link router-link to="/dashboard" class="py-3">
                    <template #prepend>
                        <img :src="DashboardIcon" alt="Dashboard" class="mr-3">
                    </template>
                    <v-list-item-title class="text-body-2">
                        Overview
                    </v-list-item-title>
                </v-list-item>

                <v-list-item link router-link to="/buckets">
                    <template #prepend>
                        <img :src="BucketIcon" alt="Buckets" class="mr-3">
                    </template>
                    <v-list-item-title class="text-body-2">
                        Buckets
                    </v-list-item-title>
                </v-list-item>

                <v-list-item link router-link to="/bucket">
                    <template #prepend>
                        <img :src="FolderIcon" alt="Demo Bucket" class="mr-3">
                    </template>
                    <v-list-item-title class="text-body-2">
                        Browse
                    </v-list-item-title>
                </v-list-item>

                <v-list-item link router-link to="/access">
                    <template #prepend>
                        <img :src="AccessIcon" alt="Access" class="mr-3">
                    </template>
                    <v-list-item-title class="text-body-2">
                        Access
                    </v-list-item-title>
                </v-list-item>

                <v-list-item link router-link to="/team">
                    <template #prepend>
                        <img :src="TeamIcon" alt="Team" class="mr-3">
                    </template>
                    <v-list-item-title class="text-body-2">
                        Team
                    </v-list-item-title>
                </v-list-item>

                <v-divider class="my-2" />

                <v-list-item link>
                    <v-menu activator="parent" location="end" transition="scale-transition">
                        <v-card min-width="300">
                            <v-list>
                                <v-list-item link class="py-3">
                                    <template #prepend>
                                        <img :src="DocsIcon" alt="Docs" class="mr-3">
                                    </template>
                                    <v-list-item-title class="text-body-2">
                                        Docs
                                    </v-list-item-title>
                                    <v-list-item-subtitle>
                                        <small>Read the documentation.</small>
                                    </v-list-item-subtitle>
                                </v-list-item>
                                <v-divider />
                                <v-list-item link class="py-3">
                                    <template #prepend>
                                        <img :src="ForumIcon" alt="Forum" class="mr-3">
                                    </template>
                                    <v-list-item-title class="text-body-2">
                                        Forum
                                    </v-list-item-title>
                                    <v-list-item-subtitle>
                                        <small>Join our global community.</small>
                                    </v-list-item-subtitle>
                                </v-list-item>
                                <v-divider />
                                <v-list-item link class="py-3">
                                    <template #prepend>
                                        <img :src="SupportIcon" alt="Support" class="mr-3">
                                    </template>
                                    <v-list-item-title class="text-body-2">
                                        Support
                                    </v-list-item-title>
                                    <v-list-item-subtitle>
                                        <small>Get support for Storj.</small>
                                    </v-list-item-subtitle>
                                </v-list-item>
                            </v-list>
                        </v-card>
                    </v-menu>

                    <template #prepend>
                        <img :src="ResourcesIcon" alt="Resources" class="mr-3">
                    </template>
                    <v-list-item-title class="text-body-2">
                        Resources
                    </v-list-item-title>
                    <template #append>
                        <img :src="RightIcon" alt="Project" width="10">
                    </template>
                </v-list-item>

                <v-list-item link>
                    <template #prepend>
                        <img :src="QuickstartIcon" alt="Quickstart" class="mr-3">
                    </template>
                    <v-list-item-title class="text-body-2">
                        Quickstart
                    </v-list-item-title>
                    <template #append>
                        <img :src="RightIcon" alt="Project" width="10">
                    </template>
                </v-list-item>

                <v-divider class="my-2" />

                <v-list-item link router-link to="/design-library">
                    <template #prepend>
                        <img :src="BookmarkIcon" alt="Design Library" class="mr-3">
                    </template>
                    <v-list-item-title class="text-body-2">
                        Design Library
                    </v-list-item-title>
                </v-list-item>
            </v-list>
        </v-sheet>
    </v-navigation-drawer>
</template>

<script setup lang="ts">
import { onBeforeMount, ref, watch } from 'vue';
import { useTheme } from 'vuetify';

import Logo from '@poc/assets/logo.svg?url';
import LogoWhite from '@poc/assets/logo-white.svg?url';
import AccountIcon from '@poc/assets/icon-account.svg?url';
import SatelliteIcon from '@poc/assets/icon-satellite.svg?url';
import UpgradeIcon from '@poc/assets/icon-upgrade.svg?url';
import SettingsIcon from '@poc/assets/icon-settings.svg?url';
import CardIcon from '@poc/assets/icon-card.svg?url';
import LogoutIcon from '@poc/assets/icon-logout.svg?url';
import ProjectsIcon from '@poc/assets/icon-projects.svg?url';
import PlusIcon from '@poc/assets/icon-plus.svg?url';
import LockIcon from '@poc/assets/icon-lock.svg?url';
import ProjectIcon from '@poc/assets/icon-project.svg?url';
import RightIcon from '@poc/assets/icon-right.svg?url';
import QuickstartIcon from '@poc/assets/icon-quickstart.svg?url';
import BookmarkIcon from '@poc/assets/icon-bookmark.svg?url';
import DashboardIcon from '@poc/assets/icon-dashboard.svg?url';
import BucketIcon from '@poc/assets/icon-bucket.svg?url';
import FolderIcon from '@poc/assets/icon-folder.svg?url';
import AccessIcon from '@poc/assets/icon-access.svg?url';
import TeamIcon from '@poc/assets/icon-team.svg?url';
import DocsIcon from '@poc/assets/icon-docs.svg?url';
import ForumIcon from '@poc/assets/icon-forum.svg?url';
import SupportIcon from '@poc/assets/icon-support.svg?url';
import ResourcesIcon from '@poc/assets/icon-resources.svg?url';

const theme = useTheme();

const drawer = ref<boolean>(true);
const menu = ref<boolean>(false);
const activeTheme = ref<number | null>(null);

function toggleTheme(newTheme) {
    if ((newTheme === 'dark' && theme.global.current.value.dark) || (newTheme === 'light' && !theme.global.current.value.dark)) {
        return;
    }
    theme.global.name.value = newTheme;
}

watch(() => theme.global.current.value.dark, (newVal) => {
    activeTheme.value = newVal ? 1 : 0;
});

onBeforeMount(() => {
    activeTheme.value = theme.global.current.value.dark ? 1 : 0;
});
</script>

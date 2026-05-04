// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app-bar :elevation="0">
        <v-app-bar-nav-icon
            variant="text" color="default" class="mr-1" size="small" density="comfortable"
            @click.stop="!mdAndUp ? drawer = !drawer : rail = !rail"
        />

        <div class="d-flex align-center mx-1">
            <router-link v-if="featureFlags.dashboard" to="/dashboard" class="d-flex align-center">
                <img :src="logoSrc" :alt="`${brandName} Logo`" class="logo-img">
            </router-link>
            <img v-else :src="logoSrc" :alt="`${brandName} Logo`" class="logo-img">
            <v-chip label size="x-small" color="primary" class="ml-2 font-weight-bold">ADMIN</v-chip>
        </div>

        <template #append>
            <v-btn
                class="mr-2"
                variant="outlined"
                color="default"
                rounded="lg"
                :icon="Search"
                @click="globalSearch = true"
            />
            <v-menu offset-y width="200" class="rounded-xl">
                <template #activator="{ props: activatorProps }">
                    <v-btn
                        class="mr-2"
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

            <v-menu offset-y class="rounded-xl">
                <template
                    v-if="featureFlags.switchSatellite"
                    #activator="{ props }"
                >
                    <v-btn
                        class="mr-2"
                        v-bind="props"
                        variant="outlined"
                        color="default"
                        rounded="lg"
                    >
                        <template #append>
                            <v-icon :icon="ChevronDown" />
                        </template>
                        North America 1
                    </v-btn>
                </template>

                <v-list class="px-1">
                    <v-list-item rounded="lg">
                        <template #prepend>
                            <img src="@/assets/icon-satellite.svg" width="16" alt="Satellite">
                        </template>
                        <v-list-item-title class="text-body-2 ml-3">Satellite</v-list-item-title>
                        <v-list-item-subtitle class="ml-3">
                            North America 1
                        </v-list-item-subtitle>
                    </v-list-item>
                </v-list>
            </v-menu>

            <v-menu v-if="oidcUser && (featureFlags.operator || featureFlags.signOut)" offset-y class="rounded-xl">
                <template #activator="{ props }">
                    <v-btn
                        class="mr-2"
                        v-bind="props"
                        variant="outlined"
                        color="default"
                        rounded="lg"
                    >
                        <template #append>
                            <v-icon :icon="ChevronDown" />
                        </template>
                        {{ oidcUser ? oidcUser.email : 'Admin' }}
                    </v-btn>
                </template>

                <v-list class="px-1">
                    <v-list-item rounded="lg" class="py-2">
                        <v-list-item-title class="text-body-2 font-weight-bold">Roles</v-list-item-title>
                        <div v-if="oidcUser.groups?.length" class="d-flex flex-wrap gap-1 mt-1">
                            <v-chip
                                v-for="group in oidcUser.groups"
                                :key="group"
                                size="x-small"
                                color="primary"
                                label
                            >
                                {{ group }}
                            </v-chip>
                        </div>
                    </v-list-item>

                    <v-list-item v-if="featureFlags.signOut" rounded="lg" link @click="appStore.logout()">
                        <v-list-item-title class="text-body-2">
                            Sign Out
                        </v-list-item-title>
                    </v-list-item>
                </v-list>
            </v-menu>
        </template>
    </v-app-bar>

    <v-navigation-drawer v-model="drawer" :rail="mdAndUp && rail" :permanent="mdAndUp" color="surface">
        <v-sheet>
            <v-list density="compact" nav>
                <v-list-item v-if="featureFlags.switchSatellite" link class="pa-4 rounded-lg">
                    <v-menu activator="parent" location="end" transition="scale-transition">
                        <v-list class="pa-2">
                            <v-list-item link rounded="lg">
                                <template #prepend>
                                    <img src="@/assets/icon-check-color.svg" alt="Selected Project">
                                </template>
                                <v-list-item-title class="text-body-2 font-weight-bold ml-3">
                                    North America (US1)
                                </v-list-item-title>
                            </v-list-item>

                            <v-list-item link rounded="lg">
                                <!-- <template v-slot:prepend>
        <img src="@/assets/icon-check-color.svg" alt="Selected Project">
        </template> -->
                                <v-list-item-title class="text-body-2 ml-7">
                                    Europe (EU1)
                                </v-list-item-title>
                            </v-list-item>

                            <v-list-item link rounded="lg">
                                <!-- <template v-slot:prepend>
        <img src="@/assets/icon-check-color.svg" alt="Selected Project">
        </template> -->
                                <v-list-item-title class="text-body-2 ml-7">
                                    Asia-Pacific (AP1)
                                </v-list-item-title>
                            </v-list-item>

                            <v-divider class="my-2" />

                            <v-list-item link rounded="lg">
                                <template #prepend>
                                    <img src="@/assets/icon-settings.svg" alt="Satellite Settings">
                                </template>
                                <v-list-item-title class="text-body-2 ml-3">
                                    Satellite Settings
                                </v-list-item-title>
                            </v-list-item>
                        </v-list>
                    </v-menu>
                    <template #prepend>
                        <img src="@/assets/icon-satellite.svg" alt="Satellite">
                    </template>
                    <v-list-item-title link class="text-body-2 ml-3">
                        Satellite
                    </v-list-item-title>
                    <v-list-item-subtitle class="ml-3">
                        North America US1
                    </v-list-item-subtitle>
                    <template #append>
                        <img src="@/assets/icon-right.svg" alt="Project" width="10">
                    </template>
                </v-list-item>

                <v-list-item v-if="featureFlags.dashboard" link router-link to="/dashboard" class="my-1 py-3" rounded="lg">
                    <template #prepend>
                        <img src="@/assets/icon-dashboard.svg" alt="Dashboard">
                    </template>
                    <v-list-item-title class="text-body-2 ml-3">
                        Dashboard
                    </v-list-item-title>
                </v-list-item>

                <v-list-item
                    v-if="featureFlags.account.search"
                    link router-link
                    :to="{ name: ROUTES.Accounts.name }"
                    rounded="lg"
                    title="Accounts"
                    :prepend-icon="UserRoundSearch"
                />

                <v-list-item
                    v-if="featureFlags.account.createRegToken"
                    link
                    rounded="lg"
                    title="Create Reg Token"
                    :prepend-icon="Key"
                    @click="showCreateRegTokenDialog = true"
                />

                <v-list-item
                    v-if="featureFlags.access.inspect"
                    link
                    rounded="lg"
                    title="Inspect Access"
                    :prepend-icon="InspectIcon"
                    @click="showInspectAccessDialog = true"
                />

                <v-list-item v-if="featureFlags.project.list" link router-link to="/projects" class="my-1" rounded="lg">
                    <template #prepend>
                        <img src="@/assets/icon-project.svg" alt="Projects">
                    </template>
                    <v-list-item-title class="text-body-2 ml-3">
                        Projects
                    </v-list-item-title>
                </v-list-item>
            </v-list>
        </v-sheet>
    </v-navigation-drawer>

    <FullScreenLoader :model-value="appStore.state.loading" />
    <GlobalSearchDialog v-model="globalSearch" />
    <CreateRegistrationTokenDialog v-model="showCreateRegTokenDialog" />
    <AccessInspectDialog v-model="showInspectAccessDialog" />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import {
    VAppBar,
    VAppBarNavIcon,
    VBtn,
    VChip,
    VDivider,
    VIcon,
    VList,
    VListItem,
    VListItemSubtitle,
    VListItemTitle,
    VMenu,
    VNavigationDrawer,
    VSheet,
} from 'vuetify/components';
import { useDisplay } from 'vuetify';
import {
    ChevronDown,
    InspectIcon,
    Key,
    Monitor,
    MoonStar,
    Search,
    Smartphone,
    Sun,
    UserRoundSearch,
} from 'lucide-vue-next';

import { FeatureFlags } from '@/api/client.gen';
import { OIDCUser, useAppStore } from '@/store/app';
import { useThemeStore } from '@/store/theme';
import { ROUTES } from '@/router';
import { defaultBrandingName, LogoKey } from '@/types/branding';

import FullScreenLoader from '@/components/FullScreenLoader.vue';
import GlobalSearchDialog from '@/components/GlobalSearchDialog.vue';
import CreateRegistrationTokenDialog from '@/components/CreateRegistrationTokenDialog.vue';
import AccessInspectDialog from '@/components/AccessInspectDialog.vue';

import logoDark from '@/assets/logo-dark.svg';
import logoLight from '@/assets/logo.svg';

const appStore = useAppStore();
const themeStore = useThemeStore();
const { mdAndUp, smAndDown } = useDisplay();

const drawer = ref<boolean>(true);
const rail = ref<boolean>(true);
const globalSearch = ref<boolean>(false);
const showCreateRegTokenDialog = ref<boolean>(false);
const showInspectAccessDialog = ref<boolean>(false);

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

const logoSrc = computed<string>(() => {
    const logoURLs = appStore.state.settings?.admin?.branding?.logoUrls;
    if (logoURLs) {
        const key = themeStore.globalTheme?.dark ? LogoKey.FullDark : LogoKey.FullLight;
        return logoURLs[key] || (themeStore.globalTheme?.dark ? logoDark : logoLight);
    }
    return themeStore.globalTheme?.dark ? logoDark : logoLight;
});

const brandName = computed<string>(() =>
    appStore.state.settings?.admin?.branding?.name ?? defaultBrandingName,
);

const featureFlags = computed(() => appStore.state.settings.admin.features as FeatureFlags);
const oidcUser = computed<OIDCUser | null>(() => appStore.state.oidcUser);
</script>

<style scoped lang="scss">
.logo-img {
    height: 24px;
    width: auto;
    max-width: 120px;
    display: block;
}
</style>

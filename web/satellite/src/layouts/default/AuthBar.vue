// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app-bar :elevation="0" border="0" class="bg-background no-shadow">
        <template #prepend>
            <div class="d-flex flex-row align-center ml-2 ml-sm-3 mr-1 mt-n1">
                <img
                    v-if="themeStore.globalTheme?.dark"
                    :src="configStore.darkLogo"
                    width="120"
                    alt="Logo"
                >
                <img
                    v-else
                    :src="configStore.logo"
                    width="120"
                    alt="Logo"
                >
                <template v-if="partnerConfig && partnerConfig.partnerLogoTopUrl && route.name === ROUTES.Signup.name">
                    <p class="mx-1">+</p>
                    <a :href="partnerConfig.partnerUrl">
                        <img
                            :src="partnerConfig.partnerLogoTopUrl"
                            height="23"
                            width="auto"
                            :alt="partnerConfig.name + ' logo'"
                            class="rounded mt-2 white-background"
                        >
                    </a>
                </template>
            </div>
        </template>
        <template #append>
            <v-menu offset-y width="200" class="rounded-xl">
                <template #activator="{ props: activatorProps }">
                    <v-btn
                        v-bind="activatorProps"
                        variant="outlined"
                        color="default"
                        rounded="lg"
                        class="mr-2"
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
        </template>
    </v-app-bar>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue';
import { useRoute } from 'vue-router';
import { VAppBar, VBtn, VList, VListItem, VListItemTitle, VMenu } from 'vuetify/components';
import { Sun, MoonStar, Monitor, Smartphone } from 'lucide-vue-next';
import { useDisplay } from 'vuetify';

import { PartnerConfig } from '@/types/partners';
import { ROUTES } from '@/router';
import { useThemeStore } from '@/store/modules/themeStore';
import { useConfigStore } from '@/store/modules/configStore';

const configStore = useConfigStore();
const themeStore = useThemeStore();

const route = useRoute();
const { smAndDown } = useDisplay();

const partnerConfig = computed<PartnerConfig | null>(() =>
    (configStore.signupConfig.get(route.query.partner?.toString() ?? '') ?? null) as PartnerConfig | null,
);

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

watch(() => route.query.partner?.toString(), async (value) => {
    if (!value) return;
    try {
        await configStore.getPartnerSignupConfig(value);
    } catch { /* empty */ }
}, { immediate: true });
</script>

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app-bar :elevation="0" border="0" class="bg-background no-shadow">
        <template #prepend>
            <div class="d-flex flex-row align-center ml-2 mr-1 mt-n1">
                <img
                    v-if="themeStore.globalTheme?.dark"
                    src="@/assets/logo-dark.svg"
                    height="23"
                    width="auto"
                    alt="Storj Logo"
                >
                <img
                    v-else
                    src="@/assets/logo.svg"
                    height="23"
                    width="auto"
                    alt="Storj Logo"
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
                        size="small"
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
        </template>
    </v-app-bar>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref } from 'vue';
import { useRoute } from 'vue-router';
import { VAppBar, VBtn, VList, VListItem, VListItemTitle, VMenu } from 'vuetify/components';
import { Sun, MoonStar, Monitor, Smartphone } from 'lucide-vue-next';
import { useDisplay } from 'vuetify';

import { PartnerConfig } from '@/types/partners';
import { ROUTES } from '@/router';
import { useThemeStore } from '@/store/modules/themeStore';

const route = useRoute();
const themeStore = useThemeStore();
const { smAndDown } = useDisplay();

const partnerConfig = ref<PartnerConfig | null>(null);

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

onBeforeMount(async () => {
    // if on signup page, and has partner in route, see if there is a partner config to display logo
    if (route.name !== ROUTES.Signup.name) {
        return;
    }
    let partner = '';
    if (route.query.partner) {
        partner = route.query.partner.toString();
    }
    // If partner.value is true, attempt to load the partner-specific configuration
    if (partner !== '') {
        try {
            const config = (await import('@/configs/registrationViewConfig.json')).default;
            partnerConfig.value = config[partner];
            // eslint-disable-next-line no-empty
        } catch {}
    }
});
</script>

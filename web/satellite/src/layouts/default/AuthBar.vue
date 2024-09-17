// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app-bar :elevation="0" border="0" class="bg-background no-shadow">
        <template #prepend>
            <div class="d-flex flex-row align-center ml-2 mr-1 mt-n1">
                <img
                    v-if="theme.global.current.value.dark"
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
            <!-- Theme Toggle Light/Dark Mode -->
            <v-btn-toggle
                v-model="activeTheme"
                mandatory
                border
                inset
                density="comfortable"
                class="pa-1 bg-surface mr-1"
            >
                <v-tooltip text="Light Theme" location="bottom">
                    <template #activator="{ props }">
                        <v-btn
                            v-bind="props"
                            rounded="xl"
                            density="comfortable"
                            size="x-small"
                            class="px-4"
                            :icon="Sun"
                            aria-label="Toggle Light Theme"
                            @click="toggleTheme('light')"
                        />
                    </template>
                </v-tooltip>

                <v-tooltip text="Dark Theme" location="bottom">
                    <template #activator="{ props }">
                        <v-btn
                            v-bind="props"
                            rounded="xl"
                            density="comfortable"
                            size="x-small"
                            class="px-4"
                            :icon="MoonStar"
                            aria-label="Toggle Dark Theme"
                            @click="toggleTheme('dark')"
                        />
                    </template>
                </v-tooltip>
            </v-btn-toggle>
        </template>
    </v-app-bar>
</template>

<script setup lang="ts">
import { onBeforeMount, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { useTheme } from 'vuetify';
import { VAppBar, VBtn, VBtnToggle, VTooltip } from 'vuetify/components';
import { Sun, MoonStar } from 'lucide-vue-next';

import { PartnerConfig } from '@/types/partners';
import { ROUTES } from '@/router';

const route = useRoute();
const theme = useTheme();

const activeTheme = ref(0);
const partnerConfig = ref<PartnerConfig | null>(null);

function toggleTheme(newTheme: string): void {
    if ((newTheme === 'dark' && theme.global.current.value.dark) || (newTheme === 'light' && !theme.global.current.value.dark)) {
        return;
    }
    theme.global.name.value = newTheme;
    localStorage.setItem('theme', newTheme);  // Store the selected theme in localStorage
}

onBeforeMount(() => {
    // Check for stored theme in localStorage. If none, default to 'light'
    toggleTheme(localStorage.getItem('theme') || 'light');
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

watch(() => theme.global.current.value.dark, (newVal: boolean) => {
    activeTheme.value = newVal ? 1 : 0;
});
</script>

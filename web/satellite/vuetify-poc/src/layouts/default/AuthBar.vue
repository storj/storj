// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app-bar :elevation="0">
        <v-app-bar-title class="mr-1">
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
                    <template #activator="{ props }">
                        <v-btn
                            v-bind="props"
                            rounded="xl"
                            density="comfortable"
                            size="small"
                            class="px-4"
                            icon
                            aria-label="Toggle Light Theme"
                            @click="toggleTheme('light')"
                        >
                            <v-icon :icon="mdiWeatherSunny" height="24" width="24" />
                        </v-btn>
                    </template>
                </v-tooltip>

                <v-tooltip text="Dark Theme" location="bottom">
                    <template #activator="{ props }">
                        <v-btn
                            v-bind="props"
                            rounded="xl"
                            density="comfortable"
                            size="small"
                            class="px-4"
                            icon
                            aria-label="Toggle Dark Theme"
                            @click="toggleTheme('dark')"
                        >
                            <v-icon :icon="mdiWeatherNight" height="24" width="24" />
                        </v-btn>
                    </template>
                </v-tooltip>
            </v-btn-toggle>
        </template>
    </v-app-bar>
</template>

<script setup lang="ts">
import { useTheme } from 'vuetify';
import { onBeforeMount, ref, watch } from 'vue';
import { VAppBar, VAppBarTitle, VBtn, VBtnToggle, VIcon, VImg, VMenu, VTooltip } from 'vuetify/components';
import { mdiWeatherNight, mdiWeatherSunny } from '@mdi/js';

const theme = useTheme();
const activeTheme = ref(0);
const menu = ref(false);

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

watch(() => theme.global.current.value.dark, (newVal: boolean) => {
    activeTheme.value = newVal ? 1 : 0;
});
</script>

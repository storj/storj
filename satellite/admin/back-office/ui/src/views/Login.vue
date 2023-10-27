// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app-bar :elevation="0">
        <v-app-bar-title class="ml-4 mr-1">
            <router-link to="/dashboard">
                <v-img v-if="theme.global.current.value.dark" src="@/assets/logo-dark.svg" width="172" alt="Storj Logo" />
                <v-img v-else src="@/assets/logo.svg" width="172" alt="Storj Logo" />
            </router-link>
        </v-app-bar-title>

        <template #append>
            <!-- Theme Toggle Light/Dark Mode -->
            <v-btn-toggle v-model="activeTheme" mandatory border inset rounded="lg" density="compact">
                <v-tooltip text="Light Theme" location="bottom">
                    <template #activator="{ props }">
                        <v-btn
                            v-bind="props" icon="mdi-weather-sunny" size="x-small" class="px-4" aria-label="Toggle Light Theme"
                            @click="toggleTheme('light')"
                        />
                    </template>
                </v-tooltip>

                <v-tooltip text="Dark Theme" location="bottom">
                    <template #activator="{ props }">
                        <v-btn
                            v-bind="props" icon="mdi-weather-night" size="x-small" class="px-4" aria-label="Toggle Dark Theme"
                            @click="toggleTheme('dark')"
                        />
                    </template>
                </v-tooltip>
            </v-btn-toggle>
        </template>
    </v-app-bar>

    <v-container>
        <v-row align="center" justify="center">
            <v-col cols="12" sm="8" md="6" lg="4">
                <v-card variant="flat" class="mt-8 pa-4" rounded="xlg" border>
                    <v-card-text>
                        <h2 class="my-1">Select a satellite</h2>
                        <p>to continue to Storj Admin</p>
                        <v-select
                            v-model="selectedSatellite" label="Satellite" placeholder="Select a satellite"
                            :items="['North America US1', 'Europe EU1', 'Asia-Pacific AP1']" variant="outlined" class="mt-5" autofocus
                            required
                        />
                        <v-btn block size="large" link router-link to="/accounts" :disabled="!selectedSatellite">Continue</v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>
    </v-container>
</template>

<script lang="ts">
import { useTheme } from 'vuetify';
import {
    VAppBar,
    VAppBarTitle,
    VImg,
    VBtnToggle,
    VTooltip,
    VBtn,
    VContainer,
    VRow,
    VCol,
    VCard,
    VCardText,
    VSelect,
} from 'vuetify/components';

export default {
    components: {
        VAppBar,
        VAppBarTitle,
        VImg,
        VBtnToggle,
        VTooltip,
        VBtn,
        VContainer,
        VRow,
        VCol,
        VCard,
        VCardText,
        VSelect,
    },
    setup() {
        const theme = useTheme();
        return {
            theme,
            toggleTheme: (newTheme) => {
                if ((newTheme === 'dark' && theme.global.current.value.dark) || (newTheme === 'light' && !theme.global.current.value.dark)) {
                    return;
                }
                theme.global.name.value = newTheme;
                localStorage.setItem('theme', newTheme);  // Store the selected theme in localStorage
            },
        };
    },
    data: () => ({
        activeTheme: null,
        selectedSatellite: 'North America US1',
    }),
    watch: {
        'theme.global.current.value.dark': function (newVal) {
            this.activeTheme = newVal ? 1 : 0;
        },
    },
    mounted() {
        document.title = 'Storj Admin - Login';
    },
    created() {
    // Check for stored theme in localStorage. If none, default to 'light'
        const storedTheme = localStorage.getItem('theme') || 'light';
        this.toggleTheme(storedTheme);
        this.activeTheme = this.theme.global.current.value.dark ? 1 : 0;
    },
};
</script>
// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="theme-area">
        <v-btn-toggle v-model="activeTheme" rounded mandatory class="custom-toggle">
            <v-tooltip bottom>
                <template #activator="props">
                    <v-btn
                        x-small
                        rounded
                        fab
                        class="mr-1"
                        :value="0"
                        v-bind="props"
                        @click="toggleTheme('light')"
                    >
                        <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="theme-icon">
                            <circle cx="12" cy="12" r="4" />
                            <path d="M12 2v2" />
                            <path d="M12 20v2" />
                            <path d="m4.93 4.93 1.41 1.41" />
                            <path d="m17.66 17.66 1.41 1.41" />
                            <path d="M2 12h2" />
                            <path d="M20 12h2" />
                            <path d="m6.34 17.66-1.41 1.41" />
                            <path d="m19.07 4.93-1.41 1.41" />
                        </svg>
                    </v-btn>
                </template>
                <span>Light Theme</span>
            </v-tooltip>
            <v-tooltip bottom>
                <template #activator="props">
                    <v-btn
                        x-small
                        rounded
                        fab
                        :value="1"
                        v-bind="props"
                        @click="toggleTheme('dark')"
                    >
                        <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="theme-icon">
                            <path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9" />
                            <path d="M20 3v4" />
                            <path d="M22 5h-4" />
                        </svg>
                    </v-btn>
                </template>
                <span>Dark Theme</span>
            </v-tooltip>
        </v-btn-toggle>
    </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue';
import { VBtn, VBtnToggle, VTooltip } from 'vuetify/components';
import { useTheme } from 'vuetify';

const theme = useTheme();

const activeTheme = ref<number>(0);

function toggleTheme(newTheme: string): void {
    if (newTheme === 'dark' && !theme.global.current.value.dark) {
        theme.change('dark');
    } else if (newTheme === 'light' && theme.global.current.value.dark) {
        theme.change('light');
    }
    localStorage.setItem('theme', newTheme);
}

onMounted(() => {
    toggleTheme(localStorage.getItem('theme') || 'light');
});

watch(() => theme.global.current.value.dark, newVal => {
    activeTheme.value = newVal ? 1 : 0;
}, { immediate: true });
</script>

<style lang="scss" scoped>
.theme-area {
    margin-right: 10px;
}

.theme-icon {
    width: 17px;
    height: 17px;
}

.custom-toggle {

    .v-btn {
        border-radius: 25px;
        background-color: transparent;
        border: none;
    }

    .v-btn.theme--dark {
        border-radius: 25px;
        background-color: transparent;
        border: none;
    }

    &.v-btn-toggle--rounded {
        background-color: var(--v-background-base);
        border: 1px solid var(--v-border-base);
        padding: 5px;
    }
}
</style>

// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="loader d-flex align-center justify-center">
        <div class="loader__spinner" />
        <img v-if="logoSrc" :src="logoSrc" class="loader__icon" alt="logo">
    </div>
</template>

<script setup lang="ts">
import { useTheme } from 'vuetify';
import { computed } from 'vue';

import { useConfigStore } from '@/store/modules/configStore';

const theme = useTheme();

const configStore = useConfigStore();

const logoSrc = computed<string>(() => {
    if (theme.global.current.value.dark) {
        return configStore.smallDarkLogo;
    } else {
        return configStore.smallLogo;
    }
});
</script>

<style scoped lang="scss">
    @keyframes spin {

        from {
            transform: rotate(0deg);
        }

        to {
            transform: rotate(360deg);
        }
    }

    @keyframes colors {

        0% {
            border-top-color: rgb(var(--v-theme-primary));
        }

        100% {
            border-top-color: rgb(var(--v-theme-secondary));
        }
    }

    .loader {
        position: absolute;
        inset: 0;
        backdrop-filter: blur(4px);

        &__spinner {
            width: 140px;
            height: 140px;
            margin: auto 0;
            border: solid 1px transparent;
            border-radius: 50%;
            animation:
                spin 0.7s linear infinite,
                colors 1.5s ease-in-out infinite;
            will-change: transform, border-top-color, box-shadow;
            box-shadow: 0 0 400px rgb(0 82 255 / 10%);
            transform: translateZ(0); // Hardware acceleration
            backface-visibility: hidden; // Prevents flickering in some browsers
        }

        &__icon {
            max-width: 125px;
            max-height: 125px;
            position: absolute;
            inset: 0;
            margin: auto;
        }
    }
</style>
// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-click-outside="close" class="options-dropdown">
        <input id="theme-switch" v-model="isDarkMode" type="checkbox" @change="close">
        <label class="options-dropdown__mode" for="theme-switch">
            <SunIcon v-if="isDarkMode" class="icon" />
            <MoonIcon v-else class="icon" />
            {{ isDarkMode ? 'Light Mode' : 'Dark Mode' }}
        </label>
    </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue';

import { SNO_THEME } from '@/app/types/theme';
import { useAppStore } from '@/app/store/modules/appStore';

import MoonIcon from '@/../static/images/DarkMoon.svg';
import SunIcon from '@/../static/images/LightSun.svg';

const appStore = useAppStore();

const emit = defineEmits<{
    (e: 'closeDropdown'): void;
}>();

const isDarkMode = ref<boolean>(false);

function close(): void {
    emit('closeDropdown');
}

watch(isDarkMode, val => {
    appStore.setDarkMode(val);
    const htmlElement = document.documentElement;

    if (val) {
        localStorage.setItem('theme', SNO_THEME.DARK);
        htmlElement.setAttribute('theme', SNO_THEME.DARK);

        return;
    }

    localStorage.setItem('theme', SNO_THEME.LIGHT);
    htmlElement.setAttribute('theme', SNO_THEME.LIGHT);
});

onMounted(() => {
    const bodyElement = document.body;
    bodyElement.classList.add('app-background');

    const htmlElement = document.documentElement;
    const theme = localStorage.getItem('theme');

    if (!theme) {
        htmlElement.setAttribute('theme', SNO_THEME.LIGHT);
        isDarkMode.value = false;
        return;
    }

    htmlElement.setAttribute('theme', theme);
    isDarkMode.value = theme === SNO_THEME.DARK;
    appStore.setDarkMode(isDarkMode.value);
});
</script>

<style scoped lang="scss">
    #theme-switch {
        display: none;
    }

    .options-dropdown {
        width: 177px;
        background-color: var(--block-background-color);
        border-bottom-left-radius: 12px;
        border-bottom-right-radius: 12px;
        font-family: 'font_regular', sans-serif;
        font-size: 14px;
        color: var(--regular-text-color);
        cursor: pointer;

        &__mode {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: flex-start;
            width: calc(100% - 24px - 24px);
            height: 44px;
            padding: 0 24px;
        }

        .icon {
            margin-right: 18px;
        }
    }
</style>

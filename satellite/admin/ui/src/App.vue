// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <template v-if="appStore.state.settings">
        <router-view />
        <notifications />
    </template>

    <FullScreenLoader :model-value="!appStore.state.settings" />
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, watch } from 'vue';
import { useTheme } from 'vuetify';

import { useAppStore } from '@/store/app';
import { useNotify } from '@/composables/useNotify';
import { DARK_THEME_QUERY, useThemeStore } from '@/store/theme';
import { ColorKey, FaviconKey } from '@/types/branding';

import Notifications from '@/layouts/default/Notifications.vue';
import FullScreenLoader from '@/components/FullScreenLoader.vue';

const appStore = useAppStore();
const themeStore = useThemeStore();
const notify = useNotify();
const theme = useTheme();

const darkThemeMediaQuery = window.matchMedia(DARK_THEME_QUERY);

function onThemeChange(e: MediaQueryListEvent) {
    themeStore.setThemeLightness(!e.matches);
}

function applyBrandingTheme(): void {
    const branding = appStore.state.settings?.admin?.branding;
    if (!branding?.colors) return;
    const c = branding.colors;
    const lightTheme = theme.themes.value.light.colors;
    const darkTheme = theme.themes.value.dark.colors;

    const primaryLightColor = c[ColorKey.PrimaryLight];
    const primaryDarkColor = c[ColorKey.PrimaryDark];
    const onPrimaryLightColor = c[ColorKey.OnPrimaryLight];
    const onPrimaryDarkColor = c[ColorKey.OnPrimaryDark];
    const secondaryLightColor = c[ColorKey.SecondaryLight];
    const secondaryDarkColor = c[ColorKey.SecondaryDark];
    const onSecondaryLightColor = c[ColorKey.OnSecondaryLight];
    const onSecondaryDarkColor = c[ColorKey.OnSecondaryDark];
    const backgroundLightColor = c[ColorKey.BackgroundLight];
    const backgroundDarkColor = c[ColorKey.BackgroundDark];
    const surfaceLightColor = c[ColorKey.SurfaceLight];
    const surfaceDarkColor = c[ColorKey.SurfaceDark];
    const onSurfaceLightColor = c[ColorKey.OnSurfaceLight];
    const onSurfaceDarkColor = c[ColorKey.OnSurfaceDark];
    const successLightColor = c[ColorKey.SuccessLight];
    const successDarkColor = c[ColorKey.SuccessDark];
    const infoLightColor = c[ColorKey.InfoLight];
    const infoDarkColor = c[ColorKey.InfoDark];
    const warningLightColor = c[ColorKey.WarningLight];
    const warningDarkColor = c[ColorKey.WarningDark];

    if (primaryLightColor) lightTheme.primary = primaryLightColor;
    if (primaryDarkColor) darkTheme.primary = primaryDarkColor;
    if (onPrimaryLightColor) lightTheme['on-primary'] = onPrimaryLightColor;
    if (onPrimaryDarkColor) darkTheme['on-primary'] = onPrimaryDarkColor;
    if (secondaryLightColor) lightTheme.secondary = secondaryLightColor;
    if (secondaryDarkColor) darkTheme.secondary = secondaryDarkColor;
    if (onSecondaryLightColor) lightTheme['on-secondary'] = onSecondaryLightColor;
    if (onSecondaryDarkColor) darkTheme['on-secondary'] = onSecondaryDarkColor;
    if (backgroundLightColor) lightTheme.background = backgroundLightColor;
    if (backgroundDarkColor) darkTheme.background = backgroundDarkColor;
    if (surfaceLightColor) lightTheme.surface = surfaceLightColor;
    if (surfaceDarkColor) darkTheme.surface = surfaceDarkColor;
    if (onSurfaceLightColor) lightTheme['on-surface'] = onSurfaceLightColor;
    if (onSurfaceDarkColor) darkTheme['on-surface'] = onSurfaceDarkColor;
    if (successLightColor) lightTheme.success = successLightColor;
    if (successDarkColor) darkTheme.success = successDarkColor;
    if (infoLightColor) lightTheme.info = infoLightColor;
    if (infoDarkColor) darkTheme.info = infoDarkColor;
    if (warningLightColor) lightTheme.warning = warningLightColor;
    if (warningDarkColor) darkTheme.warning = warningDarkColor;
}

function setFavicons(): void {
    const faviconURLs = appStore.state.settings?.admin?.branding?.faviconUrls;
    if (!faviconURLs) return;

    const defs = [
        { rel: 'icon', type: 'image/png', sizes: '16x16', href: faviconURLs[FaviconKey.Small] },
        { rel: 'icon', type: 'image/png', sizes: '32x32', href: faviconURLs[FaviconKey.Large] },
        { rel: 'apple-touch-icon', sizes: '180x180', href: faviconURLs[FaviconKey.AppleTouch] },
    ];
    for (const { rel, type, sizes, href } of defs) {
        if (!href) continue;
        let tag = document.querySelector(`link[rel="${rel}"][sizes="${sizes}"]`) as HTMLLinkElement;
        if (tag) {
            tag.href = href;
        } else {
            tag = document.createElement('link');
            tag.rel = rel;
            if (type) tag.type = type;
            tag.sizes = sizes;
            tag.href = href;
            document.head.appendChild(tag);
        }
    }
}

watch(() => themeStore.state.name, (theme) => {
    if (theme === 'auto') {
        darkThemeMediaQuery.addEventListener('change', onThemeChange);
        return;
    }
    darkThemeMediaQuery.removeEventListener('change', onThemeChange);
}, { immediate: true });

onMounted(async () => {
    try {
        await Promise.all([
            appStore.getSettings(),
            appStore.getPlacements(),
            appStore.getProducts(),
        ]);
        applyBrandingTheme();
        setFavicons();
    } catch (error) {
        notify.error(`Failed to initialise app. ${error.message}`);
    }
});

onBeforeUnmount(() => {
    darkThemeMediaQuery.removeEventListener('change', onThemeChange);
});
</script>

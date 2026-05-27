// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { computed, reactive } from 'vue';
import { useTheme } from 'vuetify';

export const DARK_THEME_QUERY = '(prefers-color-scheme: dark)';

export type ThemeName = 'light' | 'dark' | 'auto';

export class ThemeState {
    private _name: ThemeName = 'auto';

    constructor() {
        this._name = localStorage.getItem('theme') as ThemeName || 'auto';
    }

    public get name(): ThemeName {
        return this._name;
    }

    set name(name: ThemeName) {
        localStorage.setItem('theme', name);
        this._name = name;
    }
}

export const useThemeStore = defineStore('theme', () => {
    const theme = useTheme();

    const state = reactive<ThemeState>(new ThemeState());
    changeTheme(state.name);

    const globalTheme = computed(() => theme.global.current.value);

    function setTheme(name: ThemeName): void {
        if (name === state.name) {
            return;
        }
        state.name = name;
        changeTheme(name);
    }

    function setThemeLightness(isLight: boolean): void {
        if (state.name !== 'auto') {
            return;
        }
        theme.change(isLight ? 'light' : 'dark');
    }

    function changeTheme(name: string): void {
        if (name === 'auto') {
            theme.change(window.matchMedia(DARK_THEME_QUERY).matches ? 'dark' : 'light');
        } else {
            theme.change(name);
        }
    }

    return {
        state,
        globalTheme,
        setTheme,
        setThemeLightness,
    };
});
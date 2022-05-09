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

<script lang="ts">
import { Component, Vue, Watch } from 'vue-property-decorator';

import MoonIcon from '@/../static/images/DarkMoon.svg';
import SunIcon from '@/../static/images/LightSun.svg';

import { APPSTATE_ACTIONS } from '@/app/store/modules/appState';
import { SNO_THEME } from '@/app/types/theme';

// @vue/component
@Component({
    components: {
        SunIcon,
        MoonIcon,
    },
})
export default class OptionsDropdown extends Vue {
    /**
     * Uses for switching mode.
     */
    public isDarkMode = false;

    /**
     * Lifecycle hook after initial render.
     * Sets theme and 'app-background' class to body.
     */
    public mounted(): void {
        const bodyElement = document.body;
        bodyElement.classList.add('app-background');

        const htmlElement = document.documentElement;
        const theme = localStorage.getItem('theme');

        if (!theme) {
            htmlElement.setAttribute('theme', SNO_THEME.LIGHT);

            return;
        }

        htmlElement.setAttribute('theme', theme);
        const isDarkMode = theme === SNO_THEME.DARK;
        this.isDarkMode = isDarkMode;
        this.$store.dispatch(APPSTATE_ACTIONS.SET_DARK_MODE, isDarkMode);
    }

    /**
     * Closes dropdown.
     */
    public close(): void {
        this.$emit('closeDropdown');
    }

    @Watch('isDarkMode')
    private changeMode(): void {
        this.$store.dispatch(APPSTATE_ACTIONS.SET_DARK_MODE, this.isDarkMode);
        const htmlElement = document.documentElement;

        if (this.isDarkMode) {
            localStorage.setItem('theme', SNO_THEME.DARK);
            htmlElement.setAttribute('theme', SNO_THEME.DARK);

            return;
        }

        localStorage.setItem('theme', SNO_THEME.LIGHT);
        htmlElement.setAttribute('theme', SNO_THEME.LIGHT);
    }
}
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


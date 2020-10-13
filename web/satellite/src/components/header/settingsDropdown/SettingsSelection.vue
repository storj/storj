// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="settings-selection" :class="{ disabled: isOnboardingTour, active: isDropdownShown }">
        <div
            class="settings-selection__toggle-container"
            @click.stop="toggleDropdown"
        >
            <h1 class="settings-selection__toggle-container__name" :class="{ white: isDropdownShown }">Settings</h1>
            <ExpandIcon
                class="settings-selection__toggle-container__expand-icon"
                :class="{ expanded: isDropdownShown }"
                alt="Arrow down (expand)"
            />
            <SettingsDropdown
                v-show="isDropdownShown"
                @close="closeDropdown"
                v-click-outside="closeDropdown"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ExpandIcon from '@/../static/images/common/BlackArrowExpand.svg';

import { RouteConfig } from '@/router';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

import SettingsDropdown from './SettingsDropdown.vue';

@Component({
    components: {
        SettingsDropdown,
        ExpandIcon,
    },
})
export default class SettingsSelection extends Vue {
    /**
     * Indicates if current route is onboarding tour.
     */
    public get isOnboardingTour(): boolean {
        return this.$route.name === RouteConfig.OnboardingTour.name;
    }

    /**
     * Indicates if settings dropdown shown.
     */
    public get isDropdownShown(): boolean {
        return this.$store.state.appStateModule.appState.isSettingsDropdownShown;
    }

    /**
     * Toggles project dropdown visibility.
     */
    public toggleDropdown(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SETTINGS_DROPDOWN);
    }

    /**
     * Closes project dropdown.
     */
    public closeDropdown(): void {
        if (!this.isDropdownShown) return;

        this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);
    }
}
</script>

<style scoped lang="scss">
    .expanded {

        .black-arrow-expand-path {
            fill: #fff !important;
        }
    }

    .settings-selection {
        background-color: #fff;
        cursor: pointer;
        margin-right: 20px;
        min-width: 130px;
        border-radius: 6px;

        &__toggle-container {
            position: relative;
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 0 16px;
            width: calc(100% - 32px);
            height: 36px;

            &__name {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 23px;
                color: #354049;
                transition: opacity 0.2s ease-in-out;
                word-break: unset;
                margin: 0;
            }

            &__expand-icon {
                margin-left: 15px;
            }
        }

        &:hover {
            background-color: #f5f6fa;

            .settings-selection__toggle-container__name {
                font-family: 'font_bold', sans-serif;
                color: #0068dc;
            }

            .black-arrow-expand-path {
                fill: #0068dc;
            }
        }
    }

    .disabled {
        opacity: 0.5;
        pointer-events: none;
        cursor: default;
    }

    .active {
        background: #2582ff !important;
    }

    .white {
        font-family: 'font_bold', sans-serif;
        color: #fff !important;
    }
</style>

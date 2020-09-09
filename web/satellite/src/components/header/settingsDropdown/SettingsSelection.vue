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

import SettingsDropdown from './SettingsDropdown.vue';

@Component({
    components: {
        SettingsDropdown,
        ExpandIcon,
    },
})
export default class SettingsSelection extends Vue {
    public isDropdownShown: boolean = false;

    /**
     * Indicates if current route is onboarding tour.
     */
    public get isOnboardingTour(): boolean {
        return this.$route.name === RouteConfig.OnboardingTour.name;
    }

    /**
     * Toggles project dropdown visibility.
     */
    public toggleDropdown(): void {
        this.isDropdownShown = !this.isDropdownShown;
    }

    /**
     * Closes project dropdown.
     */
    public closeDropdown(): void {
        this.isDropdownShown = false;
    }
}
</script>

<style scoped lang="scss">
    .settings-selection {
        background-color: #fff;
        cursor: pointer;
        margin-right: 20px;
        min-width: 130px;

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
    }

    .disabled {
        opacity: 0.5;
        pointer-events: none;
        cursor: default;
    }

    .expanded {

        .black-arrow-expand-path {
            fill: #fff;
        }
    }

    .active {
        background: #2582ff;
        border-radius: 6px;
    }

    .white {
        color: #fff;
    }

    @media screen and (max-width: 1280px) {

        .settings-selection {
            margin-right: 30px;

            &__toggle-container {
                justify-content: space-between;
                padding-left: 10px;
            }
        }
    }
</style>

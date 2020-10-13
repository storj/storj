// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="resources-selection" :class="{ disabled: isOnboardingTour, active: isDropdownShown }">
        <div
            class="resources-selection__toggle-container"
            @click.stop="toggleDropdown"
        >
            <h1 class="resources-selection__toggle-container__name" :class="{ white: isDropdownShown }">Resources</h1>
            <ExpandIcon
                class="resources-selection__toggle-container__expand-icon"
                :class="{ expanded: isDropdownShown }"
                alt="Arrow down (expand)"
            />
            <ResourcesDropdown
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

import ResourcesDropdown from './ResourcesDropdown.vue';

@Component({
    components: {
        ResourcesDropdown,
        ExpandIcon,
    },
})
export default class ResourcesSelection extends Vue {
    /**
     * Indicates if current route is onboarding tour.
     */
    public get isOnboardingTour(): boolean {
        return this.$route.name === RouteConfig.OnboardingTour.name;
    }

    /**
     * Indicates if resources dropdown shown.
     */
    public get isDropdownShown(): boolean {
        return this.$store.state.appStateModule.appState.isResourcesDropdownShown;
    }

    /**
     * Toggles resources dropdown visibility.
     */
    public toggleDropdown(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_RESOURCES_DROPDOWN);
    }

    /**
     * Closes resources dropdown.
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

    .resources-selection {
        background-color: #fff;
        cursor: pointer;
        margin-right: 20px;
        border-radius: 6px;
        min-width: 140px;

        &__toggle-container {
            position: relative;
            display: flex;
            flex-direction: row;
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

            .resources-selection__toggle-container__name {
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

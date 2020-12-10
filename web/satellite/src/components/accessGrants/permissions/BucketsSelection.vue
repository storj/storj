// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="buckets-selection">
        <div
            class="buckets-selection__toggle-container"
            :class="{ disabled: isOnboardingTour }"
            @click.stop="toggleDropdown"
        >
            <h1 class="buckets-selection__toggle-container__name">{{ selectionLabel }}</h1>
            <ExpandIcon
                class="buckets-selection__toggle-container__expand-icon"
                alt="Arrow down (expand)"
            />
        </div>
        <BucketsDropdown
            v-if="isDropdownShown"
            @close="closeDropdown"
            v-click-outside="closeDropdown"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BucketsDropdown from '@/components/accessGrants/permissions/BucketsDropdown.vue';

import ExpandIcon from '@/../static/images/common/BlackArrowExpand.svg';

import { RouteConfig } from '@/router';

@Component({
    components: {
        ExpandIcon,
        BucketsDropdown,
    },
})
export default class BucketsSelection extends Vue {
    public isDropdownShown: boolean = false;

    /**
     * Toggles dropdown visibility.
     */
    public toggleDropdown(): void {
        if (this.isOnboardingTour) return;

        this.isDropdownShown = !this.isDropdownShown;
    }

    /**
     * Closes dropdown.
     */
    public closeDropdown(): void {
        this.isDropdownShown = false;
    }

    /**
     * Indicates if current route is onboarding tour.
     */
    public get isOnboardingTour(): boolean {
        return this.$route.path.includes(RouteConfig.OnboardingTour.path);
    }

    /**
     * Returns selection options (all or items count).
     */
    public get selectionLabel(): string {
        const ALL_SELECTED = 'All';

        if (!this.storedBucketNames.length) {
            return ALL_SELECTED;
        }

        return this.storedBucketNames.length.toString();
    }

    /**
     * Returns stored selected bucket names.
     */
    private get storedBucketNames(): string[] {
        return this.$store.state.accessGrantsModule.selectedBucketNames;
    }
}
</script>

<style scoped lang="scss">
    .buckets-selection {
        background-color: #fff;
        cursor: pointer;
        margin-left: 20px;
        border-radius: 6px;
        border: 1px solid rgba(56, 75, 101, 0.4);
        font-family: 'font_regular', sans-serif;
        width: 235px;
        position: relative;

        &__toggle-container {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 15px 20px;
            width: calc(100% - 40px);
            border-radius: 6px;

            &__name {
                font-style: normal;
                font-weight: normal;
                font-size: 16px;
                line-height: 21px;
                color: #384b65;
                margin: 0;
            }
        }
    }

    .disabled {
        pointer-events: none;
        background: #f5f6fa;
        border: 1px solid rgba(56, 75, 101, 0.4);
    }
</style>

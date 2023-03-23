// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="buckets-selection">
        <div
            class="buckets-selection__toggle-container"
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
            :show-scrollbar="showScrollbar"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue, Prop } from 'vue-property-decorator';

import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { APP_STATE_DROPDOWNS } from '@/utils/constants/appStatePopUps';

import BucketsDropdown from '@/components/onboardingTour/steps/cliFlow/permissions/BucketsDropdown.vue';

import ExpandIcon from '@/../static/images/common/BlackArrowExpand.svg';

// @vue/component
@Component({
    components: {
        ExpandIcon,
        BucketsDropdown,
    },
})
export default class BucketsSelection extends Vue {
    @Prop({ default: false })
    private readonly showScrollbar: boolean;
    /**
     * Toggles dropdown visibility.
     */
    public toggleDropdown(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_ACTIVE_DROPDOWN, APP_STATE_DROPDOWNS.BUCKET_NAMES);
    }

    /**
     * Indicates if dropdown is shown.
     */
    public get isDropdownShown(): boolean {
        return this.$store.state.appStateModule.viewsState.activeDropdown === APP_STATE_DROPDOWNS.BUCKET_NAMES;
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
        border: 1px solid rgb(56 75 101 / 40%);
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
</style>

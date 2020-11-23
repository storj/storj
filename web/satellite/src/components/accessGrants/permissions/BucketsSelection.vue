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
            <BucketsDropdown
                v-show="isDropdownShown"
                @close="closeDropdown"
                v-click-outside="closeDropdown"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BucketsDropdown from '@/components/accessGrants/permissions/BucketsDropdown.vue';

import ExpandIcon from '@/../static/images/common/BlackArrowExpand.svg';

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
        this.isDropdownShown = !this.isDropdownShown;
    }

    /**
     * Closes dropdown.
     */
    public closeDropdown(): void {
        this.isDropdownShown = false;
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

        &__toggle-container {
            position: relative;
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 15px 20px;
            width: calc(100% - 40px);

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

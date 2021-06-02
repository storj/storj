// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="bandwidth">
        <h1 class="bandwidth__title">Bandwidth & Disk</h1>
        <div class="bandwidth__dropdowns">
            <node-selection-dropdown />
            <satellite-selection-dropdown />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import NodeSelectionDropdown from '@/app/components/common/NodeSelectionDropdown.vue';
import SatelliteSelectionDropdown from '@/app/components/common/SatelliteSelectionDropdown.vue';

import { UnauthorizedError } from '@/api';

@Component({
    components: {
        NodeSelectionDropdown,
        SatelliteSelectionDropdown,
    },
})
export default class BandwidthPage extends Vue {
    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch('nodes/fetch');
        } catch (error) {
            if (error instanceof UnauthorizedError) {
                // TODO: redirect to login screen.
            }

            // TODO: notify error
        }
    }
}
</script>

<style lang="scss" scoped>
    .bandwidth {
        box-sizing: border-box;
        padding: 60px;
        height: 100%;
        overflow-y: auto;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            color: var(--c-title);
            margin-bottom: 44px;
        }

        &__dropdowns {
            display: flex;
            align-items: center;
            justify-content: flex-start;
            width: 70%;

            & > *:first-of-type {
                margin-right: 20px;
            }

            .dropdown {
                max-width: unset;
            }
        }
    }
</style>

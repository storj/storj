// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="satellite-selection-toggle-container" v-if="satellites" @click="toggleDropDown">
        <p>{{selectedSatellite ? selectedSatellite : 'All satellites'}}</p>
        <svg width="8" height="4" viewBox="0 0 8 4" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M4.66343 0.268927C4.29704 -0.0911446 3.70059 -0.0881362 3.33763 0.268927L0.20204 3.34916C-0.16435 3.70848 -0.0186405 4 0.52753 4L7.47359 4C8.01976 4 8.16292 3.70548 7.79908 3.34916L4.66343 0.268927Z" fill="#535F77"/>
        </svg>
        <SatelliteSelectionDropdown v-if="isPopupShown"/>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import SatelliteSelectionDropdown from './SatelliteSelectionDropdown.vue';
    import { APPSTATE_ACTIONS } from '@/utils/constants';

    @Component ({
        props: {
            label: String
        },
        methods: {
            toggleDropDown: function (): void {
                this.$store.dispatch(APPSTATE_ACTIONS.TOGGLE_SATELLITE_SELECTION);
            },
        },
        components: {
            SatelliteSelectionDropdown,
        },
        computed: {
            satellites: function () {
                return this.$store.state.nodeModule.satellites;
            },
            selectedSatellite: function () {
                return this.$store.state.nodeModule.selectedSatellite;
            },
            isPopupShown: function (): boolean {
                return this.$store.state.appStateModule.isSatelliteSelectionShown;
            }
        }
    })

    export default class SatelliteSelectionContainer extends Vue {
    }
</script>

<style lang="scss">
    .satellite-selection-toggle-container {
        width: 168px;
        height: 44px;
        display: flex;
        justify-content: flex-start;
        align-items: center;
        background-color: #FFFFFF;
        border: 1px solid #E8E8E8;
        border-radius: 12px;
        padding: 0 14px 0 14px;
        position: relative;
        font-size: 14px;
        margin-left: 24px;
        cursor: pointer;

        b {
            margin-right: 3px;
        }

        svg {
            position: absolute;
            right: 14px;
        }
    }
</style>

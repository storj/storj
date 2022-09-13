// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div id="app">
        <router-view />
        <!-- Area for displaying notification -->
        <NotificationArea />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { PartneredSatellite } from '@/types/common';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { MetaUtils } from '@/utils/meta';

import NotificationArea from '@/components/notifications/NotificationArea.vue';

// @vue/component
@Component({
    components: {
        NotificationArea,
    },
})
export default class App extends Vue {
    /**
     * Lifecycle hook after initial render.
     * Sets up variables from meta tags from config such satellite name, etc.
     */
    public mounted(): void {
        const satelliteName = MetaUtils.getMetaContent('satellite-name');
        const partneredSatellitesData = MetaUtils.getMetaContent('partnered-satellites');
        let partneredSatellitesJSON = [];
        if (partneredSatellitesData) {
            partneredSatellitesJSON = JSON.parse(partneredSatellitesData);
        }
        const isBetaSatellite = MetaUtils.getMetaContent('is-beta-satellite');
        const couponCodeBillingUIEnabled = MetaUtils.getMetaContent('coupon-code-billing-ui-enabled');
        const couponCodeSignupUIEnabled = MetaUtils.getMetaContent('coupon-code-signup-ui-enabled');
        const isNewProjectDashboard = MetaUtils.getMetaContent('new-project-dashboard');
        const isNewObjectsFlow = MetaUtils.getMetaContent('new-objects-flow');

        if (satelliteName) {
            this.$store.dispatch(APP_STATE_ACTIONS.SET_SATELLITE_NAME, satelliteName);

            if (partneredSatellitesJSON.length) {
                const partneredSatellites: PartneredSatellite[] = [];
                partneredSatellitesJSON.forEach((sat: PartneredSatellite) => {
                    // skip current satellite
                    if (sat.name !== satelliteName) {
                        partneredSatellites.push(sat);
                    }
                });
                this.$store.dispatch(APP_STATE_ACTIONS.SET_PARTNERED_SATELLITES, partneredSatellites);
            }
        }

        if (isBetaSatellite) {
            this.$store.dispatch(APP_STATE_ACTIONS.SET_SATELLITE_STATUS, isBetaSatellite === 'true');
        }

        if (couponCodeBillingUIEnabled) {
            this.$store.dispatch(APP_STATE_ACTIONS.SET_COUPON_CODE_BILLING_UI_STATUS, couponCodeBillingUIEnabled === 'true');
        }

        if (couponCodeSignupUIEnabled) {
            this.$store.dispatch(APP_STATE_ACTIONS.SET_COUPON_CODE_SIGNUP_UI_STATUS, couponCodeSignupUIEnabled === 'true');
        }

        if (isNewProjectDashboard) {
            this.$store.dispatch(APP_STATE_ACTIONS.SET_PROJECT_DASHBOARD_STATUS, isNewProjectDashboard === 'true');
        }

        if (isNewObjectsFlow) {
            this.$store.dispatch(APP_STATE_ACTIONS.SET_OBJECTS_FLOW_STATUS, isNewObjectsFlow === 'true');
        }
    }
}
</script>

<style lang="scss">
    html {
        overflow: hidden;
        font-size: 14px;
    }

    body {
        margin: 0 !important;
        height: 100vh;
        zoom: 100%;
        overflow: hidden;
    }

    img,
    a {
        -webkit-user-drag: none;
    }

    #app {
        height: 100%;
    }

    @font-face {
        font-family: 'font_regular';
        font-style: normal;
        font-weight: 400;
        font-display: swap;
        src:
            local(''),
            url('../static/fonts/inter-v3-latin-regular.woff2') format('woff2'),
            url('../static/fonts/inter-v3-latin-regular.woff') format('woff'),
            url('../static/fonts/inter-v3-latin-regular.ttf') format('truetype');
    }

    @font-face {
        font-family: 'font_medium';
        font-style: normal;
        font-weight: 600;
        font-display: swap;
        src:
            local(''),
            url('../static/fonts/inter-v3-latin-600.woff2') format('woff2'),
            url('../static/fonts/inter-v3-latin-600.woff') format('woff'),
            url('../static/fonts/inter-v3-latin-600.ttf') format('truetype');
    }

    @font-face {
        font-family: 'font_bold';
        font-style: normal;
        font-weight: 800;
        font-display: swap;
        src:
            local(''),
            url('../static/fonts/inter-v3-latin-800.woff2') format('woff2'),
            url('../static/fonts/inter-v3-latin-800.woff') format('woff'),
            url('../static/fonts/inter-v3-latin-800.ttf') format('truetype');
    }

    a {
        text-decoration: none;
        outline: none;
        cursor: pointer;
    }

    input,
    textarea {
        font-family: inherit;
        border: 1px solid rgb(56 75 101 / 40%);
        color: #354049;
        caret-color: #2683ff;
    }

    ::-webkit-scrollbar {
        width: 4px;
    }

    ::-webkit-scrollbar-track {
        box-shadow: inset 0 0 5px #fff;
    }

    ::-webkit-scrollbar-thumb {
        background: #afb7c1;
        border-radius: 6px;
        height: 5px;
    }
</style>

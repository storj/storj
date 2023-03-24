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
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
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
        try {
            this.$store.dispatch(APP_STATE_ACTIONS.FETCH_CONFIG);
        } catch (error) {
            // TODO: Use a harsher error-handling approach when the config is necessary
            // for the frontend to function.
            this.$notify.error(error.message, null);
        }

        const satelliteName = MetaUtils.getMetaContent('satellite-name');
        const partneredSatellitesData = MetaUtils.getMetaContent('partnered-satellites');
        let partneredSatellitesJSON = [];
        if (partneredSatellitesData) {
            partneredSatellitesJSON = JSON.parse(partneredSatellitesData);
        }
        const isBetaSatellite = MetaUtils.getMetaContent('is-beta-satellite');
        const couponCodeBillingUIEnabled = MetaUtils.getMetaContent('coupon-code-billing-ui-enabled');
        const couponCodeSignupUIEnabled = MetaUtils.getMetaContent('coupon-code-signup-ui-enabled');
        const isNewAccessGrantFlow = MetaUtils.getMetaContent('new-access-grant-flow');

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

        if (isNewAccessGrantFlow) {
            this.$store.commit(APP_STATE_MUTATIONS.SET_ACCESS_GRANT_FLOW_STATUS, isNewAccessGrantFlow === 'true');
        }

        this.fixViewportHeight();
    }

    public beforeDestroy(): void {
        window.removeEventListener('resize', this.updateViewportVariable);
    }

    /**
     * Fixes the issue where view port height is taller than the visible viewport on
     * mobile Safari/Webkit. See: https://bugs.webkit.org/show_bug.cgi?id=141832
     * Specifically for us, this issue is seen in Safari and Google Chrome, both on iOS
     */
    private fixViewportHeight(): void {
        const agent = window.navigator.userAgent.toLowerCase();
        const isMobile = screen.width <= 500;
        const isIOS = agent.includes('applewebkit') && agent.includes('iphone');
        // We don't want to apply this fix on FxIOS because it introduces strange behavior
        // while not fixing the issue because it doesn't exist here.
        const isFirefoxIOS = window.navigator.userAgent.toLowerCase().includes('fxios');

        if (isMobile && isIOS && !isFirefoxIOS) {
            // Set the custom --vh variable to the root of the document.
            document.documentElement.style.setProperty('--vh', `${window.innerHeight}px`);
            window.addEventListener('resize', this.updateViewportVariable);
        }
    }

    /**
     * Update the viewport height variable "--vh".
     * This is called everytime there is a viewport change; e.g.: orientation change.
     */
    private updateViewportVariable(): void {
        document.documentElement.style.setProperty('--vh', `${window.innerHeight}px`);
    }
}
</script>

<style lang="scss">
    @import 'static/styles/variables';

    * {
        margin: 0;
        padding: 0;
    }

    html {
        overflow: hidden;
        font-size: 14px;
    }

    body {
        margin: 0 !important;
        height: var(--vh, 100vh);
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

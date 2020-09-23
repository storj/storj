// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div id="app">
        <router-view/>
        <!-- Area for displaying notification -->
        <NotificationArea/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import NotificationArea from '@/components/notifications/NotificationArea.vue';

import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { MetaUtils } from '@/utils/meta';

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
        const segmentioId = MetaUtils.getMetaContent('segment-io');

        if (satelliteName) {
            this.$store.dispatch(APP_STATE_ACTIONS.SET_SATELLITE_NAME, satelliteName);
        }

        if (segmentioId) {
            this.$segment.init(segmentioId);
        }
    }
}
</script>

<style lang="scss">
    html {
        overflow: hidden;
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

    @font-face {
        font-family: 'font_regular';
        font-display: swap;
        src: url('../static/fonts/font_regular.ttf');
    }

    @font-face {
        font-family: 'font_medium';
        font-display: swap;
        src: url('../static/fonts/font_medium.ttf');
    }

    @font-face {
        font-family: 'font_bold';
        font-display: swap;
        src: url('../static/fonts/font_bold.ttf');
    }

    a {
        text-decoration: none;
        outline: none;
        cursor: pointer;
    }

    input,
    textarea {
        font-family: inherit;
        font-weight: 600;
        border: 1px solid rgba(56, 75, 101, 0.4);
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

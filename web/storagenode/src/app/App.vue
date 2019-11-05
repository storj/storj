// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div id="app" @click="onClick">
        <router-view/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { APPSTATE_ACTIONS } from './store/modules/appState';

@Component
export default class App extends Vue {
    private ids: string[] = [
        'satelliteDropdown',
    ];

    public onClick(event: Event): void {
        let target: any = event.target;
        while (target) {
            if (this.ids.includes(target.id)) {
                return;
            }
            target = target.parentNode;
        }

        this.$store.dispatch(APPSTATE_ACTIONS.CLOSE_ALL_POPUPS);
    }
}
</script>

<style lang="scss">
    body {
        margin: 0 !important;
        font-family: 'font_regular', sans-serif;
    }

    @font-face {
        font-display: swap;
        font-family: 'font_regular';
        src: url('../../static/fonts/font_regular.ttf');
    }

    @font-face {
        font-display: swap;
        font-family: 'font_medium';
        src: url('../../static/fonts/font_medium.ttf');
    }

    @font-face {
        font-display: swap;
        font-family: 'font_bold';
        src: url('../../static/fonts/font_bold.ttf');
    }
</style>

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div id="app">
        <div class="container">
            <SNOHeader/>
            <div class="scrollable" @scroll="onScroll">
                <router-view/>
                <SNOFooter />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import SNOFooter from '@/app/components/SNOFooter.vue';
import SNOHeader from '@/app/components/SNOHeader.vue';

const elementsIdsToRemoveOnScroll: string[] = [
    'bandwidth-tooltip',
    'bandwidth-tooltip-arrow',
    'bandwidth-tooltip-point',
    'disk-space-tooltip',
    'disk-space-tooltip-arrow',
    'disk-space-tooltip-point',
    'egress-tooltip',
    'egress-tooltip-arrow',
    'egress-tooltip-point',
    'ingress-tooltip',
    'ingress-tooltip-arrow',
    'ingress-tooltip-point',
];

const elementsClassesToRemoveOnScroll: string[] = [
    'info__message-box',
    'payout-period-calendar',
    'notification-popup-container',
];

@Component({
    components: {
        SNOHeader,
        SNOFooter,
    },
})
export default class App extends Vue {
    public async beforeCreate(): Promise<void> {
        // TODO: place key to server config.
        await this.$telemetry.init('DTEcoJRlUAN2VylCWMiLrqoknW800GNO');
    }
    public onScroll(): void {
        elementsIdsToRemoveOnScroll.forEach(id => {
            this.removeElementById(id);
        });

        elementsClassesToRemoveOnScroll.forEach(className => {
            this.removeElementByClass(className);
        });
    }

    private removeElementByClass(className): void {
        const element: HTMLElement = document.querySelector(className);
        if (element) {
            element.remove();
        }
    }

    private removeElementById(id): void {
        const element: HTMLElement | null = document.getElementById(id);
        if (element) {
            element.remove();
        }
    }
}
</script>

<style lang="scss">
    @import 'static/styles/variables';

    ::-webkit-scrollbar {
        display: none;
        position: fixed;
        right: 0;
    }

    body {
        margin: 0 !important;
        position: relative;
        font-family: 'font_regular', sans-serif;
        overflow-y: hidden;
    }

    .container {
        background-color: var(--app-background-color);
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        height: auto;
    }

    .scrollable {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        padding-top: 89px;
        height: calc(100vh - 89px);
        width: 100vw;
        overflow-y: scroll;
    }

    .back-button {

        path {
            fill: var(--regular-icon-color) !important;
        }
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

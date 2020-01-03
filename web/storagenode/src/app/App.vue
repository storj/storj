// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div id="app">
        <div class="container">
            <SNOHeader/>
            <router-view/>
            <SNOFooter/>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import SNOFooter from '@/app/components/SNOFooter.vue';
import SNOHeader from '@/app/components/SNOHeader.vue';

@Component({
    components: {
        SNOHeader,
        SNOFooter,
    },
})
export default class App extends Vue {}
</script>

<style lang="scss">
    body {
        margin: 0 !important;
        position: relative;
        font-family: 'font_regular', sans-serif;
    }

    .container {
        background-color: #f4f6f9;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
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

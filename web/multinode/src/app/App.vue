// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div id="app">
        <router-view />
        <Notifications/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import Notifications from './components/notification/Notifications.vue';

// @vue/component
@Component({
    components:{
        Notifications,
    }
})
export default class App extends Vue {
}
</script>

<style lang="scss">
    @import 'static/styles/variables';

    body {
        margin: 0 !important;
        position: relative;
        overflow-y: hidden;
    }

    p,
    h1,
    h2,
    h3,
    h4 {
        margin: 0;
    }

    #app {
        width: 100vw;
        height: 100vh;
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
        font-family: 'font_semiBold';
        src: url('../../static/fonts/font_semiBold.ttf');
    }

    @font-face {
        font-display: swap;
        font-family: 'font_bold';
        src: url('../../static/fonts/font_bold.ttf');
    }
</style>

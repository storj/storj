// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div id="app">
        <router-view/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

@Component({
    data: function () {

    }
})

export default class App extends Vue {
}
</script>

<style lang="scss">
    body {
        margin: 0 !important;
    }
    @font-face {
        font-family: "font_regular";
        src: url("../static/fonts/font_regular.ttf");
    }

    @font-face {
        font-family: "font_medium";
        src: url("../static/fonts/font_medium.ttf");
    }

    @font-face {
        font-family: "font_bold";
        src: url("../static/fonts/font_bold.ttf");
    }
</style>

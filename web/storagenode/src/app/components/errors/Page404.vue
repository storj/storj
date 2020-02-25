// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./page40X.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import MainIcon from '@/../static/images/errors/404.svg';
import StorjIcon from '@/../static/images/storjIcon.svg';

@Component({
    components: {
        StorjIcon,
        MainIcon,
    },
})
// 404 component on page not found error
export default class Page404 extends Vue {}
</script>

<style scoped lang="scss">
    .error-container {
        position: fixed;
        top: 0;
        left: 0;
        z-index: 1000;
        width: 100%;
        height: 100vh;
        background-color: #fff;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        -webkit-touch-callout: none;
        -webkit-user-select: none;
        -moz-user-select: none;
        -ms-user-select: none;
        user-select: none;

        &__header {
            width: 100%;
            height: 89px;
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            display: flex;
            align-items: center;
            justify-content: center;

            &__content {
                width: 822px;
                display: flex;
                align-items: flex-start;
            }
        }

        &__title {
            font-family: 'font_medium', sans-serif;
            margin-bottom: 60px;
            font-size: 32px;
        }
    }

    .text {
        margin-left: 15px;
    }
</style>

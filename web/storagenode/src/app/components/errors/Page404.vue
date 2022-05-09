// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./page40X.html" />

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { APPSTATE_ACTIONS } from '@/app/store/modules/appState';

// @vue/component
@Component
// 404 component on page not found error
export default class Page404 extends Vue {
    public beforeMount(): void {
        this.$store.dispatch(APPSTATE_ACTIONS.SET_LOADING, false);
    }
}
</script>

<style scoped lang="scss">
    .error-container {
        position: fixed;
        top: 0;
        left: 0;
        z-index: 1000;
        width: 100%;
        height: 100vh;
        background-color: var(--error-page-background-color);
        color: var(--error-page-text-color);
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        -webkit-touch-callout: none;
        user-select: none;
        text-align: center;

        &__main-image {
            width: 320px;
            height: 311px;
            background: var(--error-page-image-path);
            background-size: contain;
        }

        &__title {
            margin-block-start: 0;
            margin-block-end: 0;
            font-family: 'font_bold', sans-serif;
            margin-top: 46px;
            font-size: 42px;
            line-height: 42px;
        }

        &__text {
            margin-block-start: 0;
            margin-block-end: 0;
            font-family: 'font_regular', sans-serif;
            margin: 25px 0 36px;
            font-size: 16px;
            line-height: 16px;
        }

        &__button {
            padding: 16px 24px;
            background: var(--error-page-button-background);
            border-radius: 6px;
            color: white;
            font-family: 'font_bold', sans-serif;
            font-size: 14px;
            line-height: 14px;
            text-decoration: none;
        }
    }

    @media screen and (max-width: 630px) {

        .error-container {
            padding: 0 27px;
            width: calc(100% - 54px);

            &__main-image {
                margin-top: 89px;
                width: 184px;
                height: 179px;
            }

            &__title {
                margin-top: 27px;
                font-size: 32px;
                line-height: 44px;
            }

            &__text {
                margin: 12px 0 20px;
                line-height: 24px;
            }
        }
    }
</style>

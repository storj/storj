// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="loader d-flex align-center justify-center">
        <div class="loader__spinner" />
        <img alt="loader" src="@/assets/icon-load.svg" class="loader__icon pb-3">
    </div>
</template>

<style scoped lang="scss">
    @keyframes rotate {

        from {
            transform: rotate(0deg);
        }

        to {
            transform: rotate(360deg);
        }
    }

    .loader {
        position: absolute;
        inset: 0;

        &__spinner {
            width: 90px;
            height: 90px;
            margin: auto 0;
            border: solid 4px var(--c-blue-3);
            border-radius: 50%;
            border-right-color: transparent;
            border-bottom-color: transparent;
            border-left-color: transparent;
            transition: all 0.5s ease-in;
            animation-name: rotate;
            animation-duration: 1s;
            animation-iteration-count: infinite;
            animation-timing-function: linear;
        }

        &__icon {
            position: absolute;
            inset: 0;
            margin: auto;
        }
    }
</style>

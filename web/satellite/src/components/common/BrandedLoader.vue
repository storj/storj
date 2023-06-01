// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="loader">
        <div class="loader__spinner" />
        <LoaderImage class="loader__icon" />
    </div>
</template>

<script setup lang="ts">
import LoaderImage from '@/../static/images/common/loadIcon.svg';
</script>

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
        display: flex;
        justify-content: center;
        align-items: center;
        position: absolute;
        inset: 0;
        background-color: #fff;

        &__spinner {
            width: 90px;
            height: 90px;
            margin: auto 0;
            border: solid 3px var(--c-blue-3);
            border-radius: 50%;
            border-right-color: transparent;
            border-bottom-color: transparent;
            border-left-color: transparent;
            transition: all 0.5s ease-in;
            animation-name: rotate;
            animation-duration: 1.2s;
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

// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <transition name="fade" mode="in-out">
        <div v-show="true" class="loading-screen">
            <StorjLogo class="logo" />
            <svg height="100" width="100" class="loader">
                <circle cx="50" cy="50" r="40" class="background" />
                <circle cx="50" cy="50" r="40" class="circle" />
            </svg>
        </div>
    </transition>
</template>

<script setup lang="ts">
import StorjLogo from '@/../static/images/LogoWithoutText.svg';
</script>

<style scoped lang="scss">
    .loading-screen {
        display: flex;
        justify-content: center;
        align-items: center;
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        height: 100vh;
        z-index: 99999;
        background-color: var(--loading-screen-background-color);
        opacity: 1;

        .logo {
            position: absolute;
            top: 80px;
            left: 50%;
            transform: translateX(-50%);
        }
    }

    .storj-logo :deep(path) {
        fill: var(--loader-logo-color);
    }

    .loader {
        transform: rotate(180deg);
    }

    .circle {
        fill: none;
        stroke: #c4c4c4;
        stroke-width: 20;
        stroke-dasharray: 250;
        stroke-dashoffset: 250;
        stroke-linecap: round;
        animation: rotate 2s linear infinite, fill 1s linear infinite;
    }

    .background {
        fill: none;
        stroke: rgb(206 206 206 / 20%);
        stroke-width: 20;
    }

    @keyframes rotate {

        50% {
            stroke-dashoffset: 0;
        }

        100% {
            stroke-dashoffset: -250;
        }
    }

    @keyframes fill {

        0%,
        100% {
            stroke: #c4c4c4;
        }

        50% {
            stroke: var(--loader-fill-color);
        }
    }

    .fade-enter-active,
    .fade-leave-active {
        transition: opacity 0.5s;
    }

    .fade-enter,
    .fade-leave-to {
        opacity: 0;
    }
</style>

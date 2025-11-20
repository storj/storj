// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="loader d-flex align-center justify-center">
        <div class="loader__spinner" />
    </div>
</template>

<style scoped lang="scss">
    @keyframes spin {

        from {
            transform: rotate(0deg);
        }

        to {
            transform: rotate(360deg);
        }
    }

    @keyframes colors {

        0% {
            border-top-color: #0052ff;
        }

        100% {
            border-top-color: #091c45;
        }
    }

    .loader {
        position: absolute;
        inset: 0;
        backdrop-filter: blur(4px);

        &__spinner {
            width: 140px;
            height: 140px;
            margin: auto 0;
            border: solid 5px transparent;
            border-radius: 50%;
            animation:
                spin 0.7s linear infinite,
                colors 1.5s ease-in-out infinite;
            will-change: transform, border-top-color, box-shadow;
            box-shadow: 0 0 400px rgb(0 82 255 / 10%);
            transform: translateZ(0); // Hardware acceleration
            backface-visibility: hidden; // Prevents flickering in some browsers
        }
    }
</style>
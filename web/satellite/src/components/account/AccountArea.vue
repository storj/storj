// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-area-container">
        <router-view />
    </div>
</template>

<script setup lang="ts"></script>

<style scoped lang="scss">
    ::-webkit-scrollbar,
    ::-webkit-scrollbar-track,
    ::-webkit-scrollbar-thumb {
        width: 4px;
        background-color: #f5f6fa;
    }

    ::-webkit-scrollbar-track-piece {
        background-color: #f5f6fa;
    }

    ::-webkit-scrollbar-thumb {
        background-color: #afb7c1;
    }

    .account-area-container {
        padding: 0 30px;
        height: 100%;
    }
</style>

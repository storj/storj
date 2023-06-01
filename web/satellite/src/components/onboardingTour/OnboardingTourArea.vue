// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="tour-area">
        <router-view />
    </div>
</template>

<script setup lang="ts"></script>

<style scoped lang="scss">
.tour-area {
    padding: 30px 0;
    box-sizing: border-box;
    width: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
}

@media screen and (width <= 760px) {

    .tour-area {
        width: 88% !important;
    }
}
</style>

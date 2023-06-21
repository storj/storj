// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <Teleport to="#app">
        <div class="overall-loader">
            <VLoader width="100px" height="100px" is-white />
        </div>
    </Teleport>
</template>

<script setup lang="ts">
import { Teleport } from 'vue';

import VLoader from '@/components/common/VLoader.vue';
</script>

<style scoped lang="scss">
.overall-loader {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: rgb(27 37 51 / 75%);
    display: flex;
    align-items: center;
    justify-content: center;
}
</style>

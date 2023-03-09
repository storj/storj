// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="loader">
        <LoaderImage :class="{ white: isWhite }" :style="style" />
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import LoaderImage from '@/../static/images/common/loader.svg';

const props = withDefaults(defineProps<{
    width?: string;
    height?: string;
    isWhite?: boolean;
}>(), {
    width: 'unset',
    height: 'unset',
    isWhite: () => false,
});

const style = computed((): Record<string, unknown> => {
    return { width: props.width, height: props.height };
});
</script>

<style scoped lang="scss">
    .loader {
        width: 100%;
        display: flex;
        justify-content: center;
    }

    .white :deep(path) {
        fill: #fff;
    }
</style>

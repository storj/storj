// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="share-container">
        <ShareButton
            v-for="opt of ShareOptions"
            :key="opt"
            :option="opt"
            :link="link"
        />
    </div>
</template>

<script setup lang="ts">
import { ShareOptions } from '@/types/browser';

import ShareButton from '@/components/common/share/ShareButton.vue';

const props = defineProps<{ link: string; }>();
</script>

<style scoped lang="scss">
    .share-container {
        width: 100%;
        display: flex;
        align-items: center;
        flex-wrap: wrap;
    }
</style>

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <span class="d-inline-flex ml-2" @click.prevent="isOpen = true">
        <icon-info />
        <v-tooltip
            v-model="isOpen"
            class="text-center"
            activator="parent"
            location="top"
            max-width="300px"
            open-delay="150"
            close-delay="150"
        >
            <slot />
        </v-tooltip>
    </span>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { VTooltip } from 'vuetify/components';

import IconInfo from '@/components/icons/IconInfo.vue';

const isOpen = ref<boolean>(false);
</script>

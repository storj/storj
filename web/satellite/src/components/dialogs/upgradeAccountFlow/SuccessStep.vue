// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <p class="pb-4">Your Pro Account has been successfully activated.</p>

    <v-btn
        block
        class="mb-2"
        @click="emit('continue')"
    >
        <template #append>
            <v-icon :icon="ArrowRight" />
        </template>
        Continue
    </v-btn>
</template>

<script setup lang="ts">
import { VBtn, VIcon } from 'vuetify/components';
import { ArrowRight } from 'lucide-vue-next';

const emit = defineEmits<{
    continue: [];
}>();
</script>

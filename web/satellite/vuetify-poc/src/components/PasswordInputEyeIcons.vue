// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-tooltip v-if="isVisible" :text="`Hide ${type}`" location="top">
        <template #activator="{ props }">
            <v-icon
                v-bind="props"
                icon="mdi-eye-outline"
                @click="() => emit('toggleVisibility')"
            />
        </template>
    </v-tooltip>
    <v-tooltip v-else :text="`Show ${type}`" location="top">
        <template #activator="{ props }">
            <v-icon
                v-bind="props"
                icon="mdi-eye-off-outline"
                @click="() => emit('toggleVisibility')"
            />
        </template>
    </v-tooltip>
</template>
<script setup lang="ts">
import { VIcon, VTooltip } from 'vuetify/components';

defineProps<{
    isVisible: boolean
    type: string
}>();

const emit = defineEmits<{
    toggleVisibility: [];
}>();
</script>

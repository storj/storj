// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-tooltip v-if="isVisible" :text="`Hide ${type}`" location="top" theme="dark">
        <template #activator="{ props }">
            <v-icon
                v-bind="props"
                :icon="Eye"
                @click="() => emit('toggleVisibility')"
            />
        </template>
    </v-tooltip>
    <v-tooltip v-else :text="`Show ${type}`" location="top" theme="dark">
        <template #activator="{ props }">
            <v-icon
                v-bind="props"
                :icon="EyeOff"
                @click="() => emit('toggleVisibility')"
            />
        </template>
    </v-tooltip>
</template>
<script setup lang="ts">
import { VIcon, VTooltip } from 'vuetify/components';
import { Eye, EyeOff } from 'lucide-vue-next';

defineProps<{
    isVisible: boolean
    type: string
}>();

const emit = defineEmits<{
    toggleVisibility: [];
}>();
</script>

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        closable
        variant="tonal"
        type="warning"
        class="mt-2 mb-4"
        border
    >
        <template #default>
            <v-row class="ma-0 flex-nowrap justify-space-between align-center">
                <p>
                    Your STORJ Token balance is low. Deposit more STORJ tokens or make sure you have a credit card
                    on file to avoid interruptions in service.
                </p>
                <p class="ml-2 text-cursor-pointer text-no-wrap text-decoration-underline" @click="() => emits('click')">{{ ctaLabel }}</p>
            </v-row>
        </template>
    </v-alert>
</template>

<script setup lang="ts">
import { VAlert, VRow } from 'vuetify/components';

defineProps<{
    ctaLabel: string
}>();

const emits = defineEmits<{
    click: [],
}>();
</script>

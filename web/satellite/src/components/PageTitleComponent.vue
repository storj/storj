// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row class="ma-0 align-center">
        <h1 class="text-h5 font-weight-bold my-2">{{ title }}</h1>
        <v-tooltip v-if="extraInfo" width="250" location="bottom">
            <template #activator="activator">
                <v-icon v-bind="activator.props" size="14" :icon="Info" class="ml-2 text-primary" />
            </template>
            <template #default>
                <p>{{ extraInfo }}</p>
            </template>
        </v-tooltip>
    </v-row>
</template>

<script setup lang="ts">
import { VIcon, VTooltip, VRow } from 'vuetify/components';
import { Info } from 'lucide-vue-next';

defineProps<{
    title: string;
    extraInfo?: string;
}>();
</script>

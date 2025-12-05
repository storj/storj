// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card :subtitle="subtitle" :to="to" class="pa-1">
        <template #title>
            <v-row class="ma-0 align-center">
                <p class="v-card-title">{{ title }}</p>
                <v-tooltip v-if="extraInfo" width="250" location="bottom">
                    <template #activator="activator">
                        <v-icon v-bind="activator.props" size="12" :icon="Info" class="ml-2 text-medium-emphasis" />
                    </template>
                    <template #default>
                        <p>{{ extraInfo }}</p>
                    </template>
                </v-tooltip>
            </v-row>
        </template>
        <v-card-text>
            <v-chip :color="color" variant="tonal" class="font-weight-bold mt-n1" :to="to">{{ data }}</v-chip>
        </v-card-text>
    </v-card>
</template>

<script setup lang="ts">
import { VCard, VCardText, VChip, VIcon, VTooltip, VRow } from 'vuetify/components';
import { Info } from 'lucide-vue-next';

withDefaults(defineProps<{
    title: string;
    subtitle: string;
    data: string;
    color?: string;
    to?: string;
    extraInfo?: string;
}>(), {
    color: 'default',
    to: undefined,
    extraInfo: undefined,
});
</script>

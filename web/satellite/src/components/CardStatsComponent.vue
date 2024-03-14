// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card :title="title" :subtitle="subtitle" variant="outlined" border rounded="xlg" :to="to">
        <v-card-text>
            <v-chip :color="color" variant="tonal" class="font-weight-bold mt-n1" :to="to">{{ data }}</v-chip>
        </v-card-text>
    </v-card>
</template>

<script setup lang="ts">
import { VCard, VCardText, VChip } from 'vuetify/components';

const props = withDefaults(defineProps<{
    title: string;
    subtitle: string;
    data: string;
    color?: string;
    to?: string;
}>(), {
    color: 'default',
    to: undefined,
});
</script>

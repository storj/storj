// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-list>
        <v-list-item>
            <template #prepend>
                <v-icon
                    v-if="!isPro"
                    color="default"
                    icon="mdi-check-circle-outline"
                    size="small"
                    class="mr-3 text-disabled"
                />
                <v-icon
                    v-else
                    color="primary"
                    icon="mdi-check-circle-outline"
                    size="small"
                    class="mr-3 opacity-100"
                />
            </template>
            <v-list-item-title>
                {{ title }}
                <v-tooltip v-if="$slots.moreInfo" max-width="200px" location="top" activator="parent">
                    <slot name="moreInfo" />
                </v-tooltip>
            </v-list-item-title>
            <v-list-item-subtitle>
                {{ info }}
            </v-list-item-subtitle>
        </v-list-item>
    </v-list>
</template>

<script setup lang="ts">
import { VList, VListItem, VListItemTitle, VListItemSubtitle, VIcon, VTooltip } from 'vuetify/components';

const props = withDefaults(defineProps<{
    isPro?: boolean;
    title: string;
    info: string;
}>(), {
    isPro: false,
    title: '',
    info: '',
});
</script>

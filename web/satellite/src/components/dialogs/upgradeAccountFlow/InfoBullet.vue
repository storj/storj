// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-list class="pb-0">
        <v-list-item class="pl-0">
            <template #prepend>
                <v-icon
                    v-if="!isPro"
                    color="default"
                    :icon="CircleCheck"
                    size="small"
                    class="text-disabled mr-n4"
                />
                <v-icon
                    v-else
                    color="primary"
                    :icon="CircleCheck"
                    size="small"
                    class="opacity-100 mr-n4"
                />
            </template>
            <v-list-item-title>
                {{ title }}
                <v-tooltip v-if="$slots.moreInfo" max-width="200px" location="top" activator="props">
                    <template #activator="{ props }">
                        <v-icon v-bind="props" size="16" :icon="Info" />
                    </template>
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
import { CircleCheck, Info } from 'lucide-vue-next';

withDefaults(defineProps<{
    isPro?: boolean;
    title?: string;
    info?: string;
}>(), {
    isPro: false,
    title: '',
    info: '',
});
</script>

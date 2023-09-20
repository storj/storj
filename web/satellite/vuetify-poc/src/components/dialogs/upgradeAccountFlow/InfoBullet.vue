// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row class="pa-0 flex-nowrap">
        <v-col class="pa-2" cols="1">
            <img v-if="!isPro" src="@/../static/images/modals/upgradeFlow/greyCheckmark.svg" alt="checkmark">
            <img v-else src="@/../static/images/modals/upgradeFlow/greenCheckmark.svg" alt="checkmark">
        </v-col>
        <v-col class="pa-2" cols="11">
            <p class="font-weight-bold">
                {{ title }}
                <v-tooltip v-if="$slots.moreInfo" max-width="200px" location="top" activator="parent">
                    <slot name="moreInfo" />
                </v-tooltip>
            </p>
            <p>{{ info }}</p>
        </v-col>
    </v-row>
</template>

<script setup lang="ts">
import { VCol, VRow, VTooltip } from 'vuetify/components';

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

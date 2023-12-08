// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card :title="title" variant="flat" :border="true" rounded="xlg">
        <v-card-item class="pt-1">
            <v-row>
                <v-col cols="12" class="pb-1">
                    <v-progress-linear :color="color" :model-value="percentage" rounded height="6" />
                </v-col>

                <v-col cols="6">
                    <p class="text-medium-emphasis">Used</p>
                    <h4>{{ onlyLimit ? "---" : format(used || 0) }}</h4>
                    <v-divider class="my-3" />
                    <p class="text-medium-emphasis">Percentage</p>
                    <h4 class="">{{ onlyLimit ? "---" : percentage+'%' }}</h4>
                </v-col>

                <v-col cols="6">
                    <p class="text-right text-medium-emphasis">Available</p>
                    <h4 class="text-right">{{ onlyLimit ? "---" : format(available) }}</h4>
                    <v-divider class="my-3" />
                    <p class="text-right text-medium-emphasis">Limit</p>
                    <h4 class="text-right">{{ format(limit) }}</h4>
                </v-col>

                <v-divider />

                <v-col>
                    <v-btn size="small" variant="outlined" color="default" class="mt-1 my-2">Change Limits</v-btn>
                </v-col>
            </v-row>
        </v-card-item>
    </v-card>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import {
    VCard,
    VCardItem,
    VRow,
    VCol,
    VProgressLinear,
    VDivider,
    VBtn,
} from 'vuetify/components';

import { Dimensions, Size } from '@/utils/bytesSize';

const props = defineProps<{
    title: string;
    isBytes?: boolean;
    onlyLimit?: boolean;
    used?: number;
    limit: number;
    color: string;
}>();

const percentage = computed((): string => {
    if (props.onlyLimit || !props.used) {
        return '0';
    }

    const p = props.used/props.limit * 100;
    return Math.round(p).toString();
});

const available = computed((): number => {
    if (props.onlyLimit || !props.used) {
        return 0;
    }

    return props.limit - props.used;
});

/**
* Returns a stringify val considering if val is expressed in bytes and it that case it returns the
* value in the best human readable memory size unit rounding down to 0 when its expressed in bytes
* and truncating the decimals when its expressed in other units.
*/
function format(val: number): string {
    if (!props.isBytes) {
        return val.toString();
    }

    const valmem =  new Size(val, 2);
    switch (valmem.label) {
    case Dimensions.Bytes:
        return '0';
    default:
        return `${valmem.formattedBytes.replace(/\.0+$/, '')}${valmem.label}`;
    }
}
</script>

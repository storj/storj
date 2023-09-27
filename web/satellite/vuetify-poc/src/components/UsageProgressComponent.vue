// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card :title="title" variant="flat" :border="true" rounded="xlg">
        <template #title>
            <v-card-title class="d-flex align-center">
                <component :is="iconComponents[icon]" v-if="icon" class="mr-2" />
                {{ title }}
            </v-card-title>
        </template>
        <v-card-item>
            <v-progress-linear color="success" :model-value="progress" rounded height="6" />
        </v-card-item>
        <v-card-item>
            <v-row>
                <v-col>
                    <h4>{{ used }}</h4>
                    <p class="text-medium-emphasis"><small>{{ limit }}</small></p>
                </v-col>
                <v-col>
                    <h4 class="text-right">{{ available }}</h4>
                    <p class="text-cursor-pointer text-right text-medium-emphasis" @click="emit('ctaClick')"><small>{{ cta }}</small></p>
                </v-col>
            </v-row>
        </v-card-item>
    </v-card>
</template>

<script setup lang="ts">
import { VCard, VCardItem, VProgressLinear, VRow, VCol, VCardTitle } from 'vuetify/components';

import IconCloud from '@poc/components/icons/IconCloud.vue';
import IconArrowDown from '@poc/components/icons/IconArrowDown.vue';
import IconGlobe from '@poc/components/icons/IconGlobe.vue';
import IconCircleCheck from '@poc/components/icons/IconCircleCheck.vue';

const props = defineProps<{
    title: string;
    progress: number;
    used: string;
    limit: string;
    available: string;
    cta: string;
    icon?: keyof typeof iconComponents;
}>();

const emit = defineEmits<{
    ctaClick: [];
}>();

const iconComponents = {
    cloud: IconCloud,
    'arrow-down': IconArrowDown,
    globe: IconGlobe,
    check: IconCircleCheck,
};
</script>

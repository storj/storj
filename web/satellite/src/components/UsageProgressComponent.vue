// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card :title="title">
        <template #title>
            <v-card-title class="d-flex align-center">
                <component :is="iconComponents[icon]" v-if="icon" class="mr-2" width="16" height="16" bold />
                {{ title }}
                <v-tooltip v-if="extraInfo || slots.extraInfo" width="250" location="bottom">
                    <template #activator="activator">
                        <v-icon v-bind="activator.props" size="16" :icon="mdiInformationOutline" class="ml-2 text-medium-emphasis" />
                    </template>
                    <template #default>
                        <template v-if="slots.extraInfo">
                            <slot name="extraInfo" />
                        </template>
                        <p v-else>{{ extraInfo }}</p>
                    </template>
                </v-tooltip>
            </v-card-title>
        </template>
        <v-card-item>
            <v-progress-linear :color="progressColor" bg-color="default" :model-value="progress" rounded height="6" />
        </v-card-item>
        <v-card-item>
            <v-row>
                <v-col>
                    <h4>{{ used }}</h4>
                    <p class="text-medium-emphasis"><small>{{ limit }}</small></p>
                </v-col>
                <v-col>
                    <h4 class="text-right">{{ available }}</h4>
                    <p class="text-right text-medium-emphasis"><a class="link" role="button" @click="emit('ctaClick')"><small>{{ cta }}</small></a></p>
                </v-col>
            </v-row>
        </v-card-item>
    </v-card>
</template>

<script setup lang="ts">
import { VCard, VCardItem, VProgressLinear, VRow, VCol, VCardTitle, VTooltip, VIcon } from 'vuetify/components';
import { Component, computed } from 'vue';
import { mdiInformationOutline } from '@mdi/js';

import IconCloud from '@/components/icons/IconCloud.vue';
import IconArrowDown from '@/components/icons/IconArrowDown.vue';
import IconGlobe from '@/components/icons/IconGlobe.vue';
import IconCircleCheck from '@/components/icons/IconCircleCheck.vue';
import IconBucket from '@/components/icons/IconBucket.vue';

const props = defineProps<{
    title: string;
    progress: number;
    used: string;
    limit: string;
    available: string;
    cta: string;
    icon?: keyof typeof iconComponents;
    extraInfo?: string;
}>();

const emit = defineEmits<{
    ctaClick: [];
}>();

const slots = defineSlots<{
    extraInfo?: Component;
}>();

const iconComponents = {
    cloud: IconCloud,
    'arrow-down': IconArrowDown,
    globe: IconGlobe,
    check: IconCircleCheck,
    bucket: IconBucket,
};

const progressColor = computed(() => {
    if (props.progress >= 100) {
        return 'error';
    } else if (props.progress >= 80) {
        return 'warning';
    } else {
        return 'success';
    }
});
</script>

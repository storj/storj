// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card :title="title" class="pa-1">
        <template #title>
            <v-card-title class="d-flex align-center">
                <v-icon v-if="icon" :icon="iconComponents[icon]" size="small" class="mr-2" color="primary" />
                {{ title }}
                <v-tooltip v-if="extraInfo || slots.extraInfo" width="250" location="bottom">
                    <template #activator="activator">
                        <v-icon v-bind="activator.props" size="12" :icon="Info" class="ml-2 text-medium-emphasis" />
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
            <v-progress-linear
                :class="{ 'no-limit-progress': noLimit }"
                :color="noLimit ? 'success' : progressColor"
                bg-color="default"
                :model-value="noLimit ? 0 : progress"
                rounded
                height="6"
            />
        </v-card-item>
        <v-card-item>
            <v-row>
                <v-col>
                    <p class="font-weight-bold text-body-2">{{ used }}</p>
                    <p class="text-medium-emphasis"><small>{{ limit }}</small></p>
                </v-col>
                <v-col>
                    <p class="text-right font-weight-bold text-body-2">{{ available }}</p>
                    <p v-if="!hideCta" class="text-right text-medium-emphasis"><a class="link" role="button" @click="emit('ctaClick')"><small>{{ cta }}</small></a></p>
                </v-col>
            </v-row>
        </v-card-item>
    </v-card>
</template>

<script setup lang="ts">
import { FunctionalComponent, computed } from 'vue';
import { VCard, VCardItem, VProgressLinear, VRow, VCol, VCardTitle, VTooltip, VIcon } from 'vuetify/components';
import { Info, Cloud, CloudDownload, Puzzle, TicketPercent } from 'lucide-vue-next';

import IconBucket from '@/components/icons/IconBucket.vue';

const props = defineProps<{
    title: string;
    progress: number;
    used: string;
    limit: string;
    available: string;
    cta: string;
    hideCta?: boolean;
    icon?: keyof typeof iconComponents;
    extraInfo?: string;
    noLimit?: boolean;
}>();

const emit = defineEmits<{
    ctaClick: [];
}>();

const slots = defineSlots<{
    extraInfo?: FunctionalComponent;
}>();

const iconComponents = {
    storage: Cloud,
    download: CloudDownload,
    segments: Puzzle,
    bucket: IconBucket,
    coupon: TicketPercent,
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

<style scoped lang="scss">
    .no-limit-progress {
        background: linear-gradient(90deg, #091C45, #2338C0, #0052FF, #0052FF, #00c6ff, #00ff6a, #00C257, #ffb018, #FF8E45, #ff4ed8, #882de3, #0052FF, #0052FF, #2338C0, #091C45);
        background-size: 200% 100%;
        animation: gradient-animation 12s linear infinite;
        transition: all 0.14s ease-in-out;
        height: 2px !important;
        margin-top: 2px;
        margin-bottom: 2px;
    }

    .no-limit-progress:hover {
        animation: gradient-animation 2s linear infinite;
        height: 6px !important;
        margin-top: 0;
        margin-bottom: 0;
    }

    @keyframes gradient-animation {

        0% {
            background-position: 0 0;
        }

        50% {
            background-position: 100% 0;
        }

        100% {
            background-position: 0 0;
        }
    }
</style>

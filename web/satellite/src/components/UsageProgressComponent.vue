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
                        <v-icon v-bind="activator.props" size="16" :icon="Info" class="ml-2 text-medium-emphasis" />
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
                    <h4>{{ used }}</h4>
                    <p class="text-medium-emphasis"><small>{{ limit }}</small></p>
                </v-col>
                <v-col>
                    <h4 class="text-right">{{ available }}</h4>
                    <p v-if="!hideCta" class="text-right text-medium-emphasis"><a class="link" role="button" @click="emit('ctaClick')"><small>{{ cta }}</small></a></p>
                </v-col>
            </v-row>
        </v-card-item>
    </v-card>
</template>

<script setup lang="ts">
import { FunctionalComponent, computed } from 'vue';
import { VCard, VCardItem, VProgressLinear, VRow, VCol, VCardTitle, VTooltip, VIcon } from 'vuetify/components';
import { Info, ArrowDownToLine } from 'lucide-vue-next';

import IconCloud from '@/components/icons/IconCloud.vue';
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
    cloud: IconCloud,
    'arrow-down': ArrowDownToLine,
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

<style scoped lang="scss">
    .no-limit-progress {
        background: linear-gradient(90deg,
        #091C45,   /* Secondary blue */
        #2338C0,   /* Dark blue */
        #0052FF,   /* Primary blue */
        #0052FF,   /* Primary blue */
        #00c6ff,   /* Cyan */
        #00ff6a,   /* Green */
        #00C257,   /* Green 2 */
        #ffb018,   /* Yellow */
        #FF8E45,   /* Orange */
        #ff4ed8,   /* Pink */
        #882de3,   /* Purple */
        #0052FF,   /* Primary blue */
        #0052FF,   /* Primary blue */
        #2338C0,   /* Dark blue */
        #091C45,   /* Secondary blue*/
    );
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

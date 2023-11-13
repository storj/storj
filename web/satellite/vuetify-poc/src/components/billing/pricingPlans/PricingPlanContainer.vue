// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="border-sm rounded-lg pa-8">
        <v-row class="ma-0" justify="center" align="center">
            <v-col cols="auto">
                <v-badge v-if="isPartner" label="Best Value" rounded="lg" content="Best Value" color="success">
                    <v-btn v-if="isPartner" density="comfortable" color="success" variant="outlined" icon>
                        <v-icon icon="mdi-cloud-outline" />
                    </v-btn>
                </v-badge>

                <v-btn v-else density="comfortable" color="grey-lighten-1" variant="outlined" icon>
                    <v-icon v-if="isPro" icon="mdi-star-outline" />
                    <v-icon v-else icon="mdi-earth" />
                </v-btn>
            </v-col>
        </v-row>

        <div class="py-4 text-center">
            <p class="font-weight-bold">{{ plan.title }}</p>
            <p>{{ plan.containerSubtitle }}</p>
        </div>

        <div class="py-4 text-center">
            <p class="mb-3">{{ plan.containerDescription }}</p>
            <!-- eslint-disable-next-line vue/no-v-html -->
            <p v-if="plan.containerFooterHTML" v-html="plan.containerFooterHTML" />
        </div>

        <v-row class="py-4" justify="center">
            <v-col class="pa-0" cols="auto">
                <v-btn
                    :variant="isFree ? 'outlined' : 'flat'"
                    :color="isPartner ? 'success' : isFree ? 'grey-lighten-1' : 'primary'"
                    @click="onActivateClick"
                >
                    <template #append>
                        <v-icon icon="mdi-arrow-right" />
                    </template>

                    {{ plan.activationButtonText || ('Activate ' + plan.title) }}
                </v-btn>
            </v-col>
        </v-row>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VBadge, VBtn, VCol, VIcon, VRow } from 'vuetify/components';

import { PricingPlanInfo, PricingPlanType } from '@/types/common';

const props = defineProps<{
    plan: PricingPlanInfo;
}>();

const emit = defineEmits<{
    select: [PricingPlanInfo];
}>();

/**
 * Sets the selected pricing plan and displays the pricing plan modal.
 */
function onActivateClick(): void {
    emit('select', props.plan);
}

const isPartner = computed((): boolean => props.plan.type === PricingPlanType.PARTNER);
const isPro = computed((): boolean => props.plan.type === PricingPlanType.PRO);
const isFree = computed((): boolean => props.plan.type === PricingPlanType.FREE);
</script>
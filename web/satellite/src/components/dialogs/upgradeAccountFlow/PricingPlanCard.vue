// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="outlined" elevation="0" rounded="xlg" class="h-100">
        <div class="h-100 d-flex flex-column justify-space-between pa-6 pa-sm-8">
            <h3 class="font-weight-black mb-1">{{ plan.planTitle }}</h3>
            <p class="mb-2 text-body-2">
                {{ plan.planSubtitle }}
            </p>

            <h5 class="mt-3 font-weight-black text-h5">{{ plan.planCost }}</h5>
            <!-- eslint-disable-next-line vue/no-v-html -->
            <p class="text-medium-emphasis text-caption" v-html="plan.planCostInfo" />

            <v-btn
                :id="id"
                :disabled="disableCta"
                :variant="isFreePlan ? 'outlined' :'flat'"
                :color="isFreePlan ? 'text-secondary' : isProPlan ? 'primary' : 'secondary'"
                class="mt-4 mb-4"
                @click="emit('ctaClick')"
            >
                {{ plan.planCTA }}
                <template v-if="!disableCta" #append>
                    <v-icon :icon="ArrowRight" />
                </template>
            </v-btn>

            <div class="text-left">
                <template v-for="(txt, index) in plan.planInfo" :key="txt">
                    <p v-if="txt" class="text-body-2 my-2">
                        <v-icon :icon="Check" size="14" class="mr-2" />
                        {{ txt }}
                    </p>

                    <v-divider v-if="txt && index < plan.planInfo.length - 1" />
                </template>
            </div>
            <v-spacer />
        </div>
    </v-card>
</template>

<script setup lang="ts">
import { VBtn, VCard, VDivider, VIcon, VSpacer } from 'vuetify/components';
import { ArrowRight, Check } from 'lucide-vue-next';
import { computed } from 'vue';

import { PricingPlanInfo, PricingPlanType } from '@/types/common';

const props = defineProps<{
    plan: PricingPlanInfo,
    id?: string,
    disableCta?: boolean,
}>();

const emit = defineEmits<{
    ctaClick: [];
}>();

const isFreePlan = computed(() => props.plan.type === PricingPlanType.FREE);
const isProPlan = computed(() => props.plan.type === PricingPlanType.PRO);
</script>

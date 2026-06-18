// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="outlined" elevation="0" rounded="xlg" class="h-100">
        <div class="h-100 d-flex flex-column justify-space-between pa-6 pa-sm-8">
            <h3 class="font-weight-black mb-1">{{ plan.planTitle }}</h3>
            <!-- eslint-disable-next-line vue/no-v-html -->
            <p class="mb-2 text-body-medium" :class="{'three-line-text': !plan.planCost}" v-html="plan.planSubtitle" />

            <h5 v-if="plan.planCost" class="mt-3 font-weight-black text-headline-small">{{ plan.planCost }}</h5>
            <!-- eslint-disable-next-line vue/no-v-html -->
            <p v-if="plan.planCostInfo" class="text-medium-emphasis text-body-small mb-4" v-html="plan.planCostInfo" />

            <div v-if="plan.planInfo.length" class="text-left">
                <template v-for="(txt, index) in plan.planInfo" :key="txt">
                    <p v-if="txt" class="text-body-medium my-2">
                        <v-icon :icon="Check" size="14" class="mr-2" />
                        {{ txt }}
                    </p>

                    <v-divider v-if="txt && index < plan.planInfo.length - 1" />
                </template>
            </div>
            <v-spacer />

            <v-btn
                :id="id"
                :disabled="disableCta"
                :variant="isFreePlan ? 'outlined' :'flat'"
                :color="isFreePlan ? 'text-secondary' : isProPlan ? 'primary' : 'secondary'"
                class="mt-4"
                @click="emit('ctaClick')"
            >
                {{ plan.planCTA }}
                <template v-if="!disableCta" #append>
                    <v-icon :icon="ArrowRight" />
                </template>
            </v-btn>
        </div>
    </v-card>
</template>

<script setup lang="ts">
import { VBtn, VCard, VDivider, VIcon, VSpacer } from 'vuetify/components';
import { ArrowRight, Check } from '@lucide/vue';
import { computed } from 'vue';

import { type PricingPlanInfo, PricingPlanType  } from '@/types/common';

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

<style lang="scss" scoped>
.three-line-text {
    line-height: 1.5;
    min-height: calc(1.5rem * 3);
}
</style>
// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card class="h-100" hover @click="onActivateClick">
        <div class="h-100 d-flex flex-column justify-space-between pa-6 pa-sm-8">
            <div>
                <div class="d-flex justify-center align-center ma-0">
                    <div>
                        <v-badge v-if="isPartner" label="Best Value" rounded="lg" content="Best Value" color="secondary">
                            <v-btn v-if="isPartner" density="comfortable" color="default" variant="outlined" icon>
                                <v-icon :icon="Gift" />
                            </v-btn>
                        </v-badge>

                        <v-btn v-else density="comfortable" color="default" variant="outlined" icon>
                            <v-icon v-if="isPro" :icon="Star" />
                            <v-icon v-else :icon="Sprout" />
                        </v-btn>
                    </div>
                </div>

                <div class="py-3 text-center">
                    <p class="font-weight-bold">{{ plan.title }}</p>
                    <p>{{ plan.containerSubtitle }}</p>
                </div>

                <div class="py-3 text-center">
                    <p class="mb-3">{{ plan.containerDescription }}</p>
                    <!-- eslint-disable-next-line vue/no-v-html -->
                    <p v-if="plan.containerFooterHTML" v-html="plan.containerFooterHTML" />
                </div>
            </div>

            <div class="d-flex justify-center py-3">
                <v-btn
                    :variant="isFree ? 'outlined' : 'flat'"
                    :color="isPartner ? 'success' : isFree ? 'default' : 'primary'"
                    @click.stop="onActivateClick"
                >
                    <template #append>
                        <v-icon :icon="ArrowRight" />
                    </template>

                    {{ plan.activationButtonText || plan.title }}
                </v-btn>
            </div>
        </div>
    </v-card>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VBadge, VBtn, VCard, VIcon } from 'vuetify/components';
import { ArrowRight, Gift, Sprout, Star } from 'lucide-vue-next';

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

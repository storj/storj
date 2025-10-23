// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card v-if="!rateAndBurst" :title="title" variant="flat" rounded="xlg">
        <template v-if="featureFlags.project.updateLimits" #append>
            <v-btn
                size="small"
                variant="outlined"
                color="default"
                @click="emit('updateLimits')"
            >
                Change Limits
            </v-btn>
        </template>
        <v-card-text class="pt-2">
            <v-row class="mb-1">
                <v-col cols="12">
                    <v-progress-linear :color="color" :model-value="percentage" rounded height="6" />
                </v-col>
            </v-row>

            <div class="d-flex justify-space-between align-center">
                <div>
                    <p class="text-medium-emphasis">Used</p>
                    <h4>{{ onlyLimit ? "---" : format(used || 0) }}</h4>
                </div>

                <div>
                    <p class="text-right text-medium-emphasis">Available</p>
                    <h4 class="text-right">{{ onlyLimit ? "---" : format(available) }}</h4>
                </div>
            </div>

            <v-divider class="my-2" />
            <div class="d-flex justify-space-between align-center">
                <div>
                    <p class="text-medium-emphasis">Percentage</p>
                    <h4 class="">{{ onlyLimit ? "---" : percentage+'%' }}</h4>
                </div>
                <div>
                    <p class="text-right text-medium-emphasis">Limit</p>
                    <h4 class="text-right">{{ format(limit) }}</h4>
                    <span class="text-right text-caption text-medium-emphasis">
                        <template v-if="userSpecified">{{ format(userSpecified) }} user specified </template>
                        <template v-else>&nbsp;</template>
                    </span>
                </div>
            </div>
        </v-card-text>
    </v-card>
    <v-card v-else :title="title" variant="flat" rounded="xlg">
        <template v-if="featureFlags.project.updateLimits" #append>
            <v-btn
                size="small"
                density="comfortable"
                variant="outlined"
                color="default"
                :icon="PenBox"
                @click="emit('updateLimits')"
            />
        </template>
        <v-card-text>
            <div class="d-flex justify-space-between align-center">
                <div>
                    <p class="text-medium-emphasis">Rate</p>
                    <p class="text-medium-emphasis">Burst</p>
                </div>
                <div>
                    <p>{{ rateAndBurst.rate || '__' }}</p>
                    <p>{{ rateAndBurst.burst || '__' }}</p>
                </div>
            </div>
        </v-card-text>
    </v-card>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VBtn, VCard, VCardText, VCol, VDivider, VProgressLinear, VRow } from 'vuetify/components';
import { PenBox } from 'lucide-vue-next';

import { FeatureFlags } from '@/api/client.gen';
import { useAppStore } from '@/store/app';
import { Dimensions, Size } from '@/utils/bytesSize';

const featureFlags = useAppStore().state.settings.admin.features as FeatureFlags;

const props = defineProps<{
    title: string;
    isBytes?: boolean;
    onlyLimit?: boolean;
    used?: number;
    limit: number | null;
    userSpecified?: number | null;
    rateAndBurst?: { rate: number | null; burst: number | null };
}>();

const emit = defineEmits<{
    (e: 'updateLimits'): void;
}>();

const color = computed((): string => {
    if (props.onlyLimit || !props.used || props.limit === null) {
        return 'success';
    }

    if (props.limit <= props.used) {
        return 'error';
    }

    const p = props.used/props.limit * 100;
    if (p < 50) {
        return 'success';
    } else if (p < 80) {
        return 'warning';
    } else {
        return 'error';
    }
});

const percentage = computed((): string => {
    if (props.onlyLimit || !props.used || props.limit === null) {
        return '0';
    }

    if (props.limit <= props.used) {
        return '100';
    }

    const p = props.used/props.limit * 100;
    return Math.round(p).toString();
});

const available = computed((): number => {
    if (props.onlyLimit || !props.used || props.limit === null) {
        return 0;
    }

    if (props.limit <= props.used) {
        return 0;
    }

    return props.limit - props.used;
});

/**
* Returns a stringify val considering if val is expressed in bytes and it that case it returns the
* value in the best human readable memory size unit rounding down to 0 when its expressed in bytes
* and truncating the decimals when its expressed in other units.
*/
function format(val: number | null): string {
    if (val === null || val === undefined) {
        return 'No Explicit Limit Set';
    }
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

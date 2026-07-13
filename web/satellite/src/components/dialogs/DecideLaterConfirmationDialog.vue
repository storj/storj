// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="480px"
        transition="fade-transition"
    >
        <v-card>
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <component :is="CircleQuestionMark" :size="18" color="orange" />
                    </v-sheet>
                </template>

                <v-card-title class="font-weight-bold">
                    Not ready to decide yet?
                </v-card-title>

                <template #append>
                    <v-btn
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-card-text>
                {{ description }}
            </v-card-text>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            color="default"
                            variant="outlined"
                            block
                            @click="model = false"
                        >
                            Go back
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary"
                            variant="flat"
                            block
                            @click="emit('confirm')"
                        >
                            Confirm
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardText,
    VCardTitle,
    VCol,
    VDialog,
    VDivider,
    VRow,
    VSheet,
} from 'vuetify/components';
import { CircleQuestionMark, X } from '@lucide/vue';

import { useConfigStore } from '@/store/modules/configStore';
import { formatConfigDate, optOutDeadline } from '@/types/pricingOptIn';

const configStore = useConfigStore();

const model = defineModel<boolean>({ required: true });
const emit = defineEmits<{ confirm: [] }>();

const description = computed<string>(() => {
    const config = configStore.state.config;
    const freezeDate = formatConfigDate(config.optOutFreezeDate);
    const optOutBy = formatConfigDate(optOutDeadline(config.optOutFreezeDate));
    const graceDays = config.optOutFreezeGracePeriodDays;

    if (config.optOutFreezeOptedOutOnly) {
        const intro = `The new pricing already applies to your account as of ${formatConfigDate(config.newPricingEffectiveDate)} — no action is needed to keep using ${configStore.brandName}.`;
        if (!freezeDate) {
            return `${intro} If you'd prefer not to accept it, you can opt out at any time.`;
        }
        return `${intro} If you'd prefer not to accept it, you can opt out until ${optOutBy}. Opted-out accounts will be frozen on ${freezeDate}, with data deleted after ${graceDays} days.`;
    }
    if (!freezeDate) {
        return `You can decide later, but if you don't accept the new pricing, your account will be scheduled to be frozen. Once frozen, you won't be able to access your data unless you accept the new pricing. Frozen accounts are permanently deleted after ${graceDays} days.`;
    }
    return `You can decide later, but if you don't accept or opt out by ${optOutBy}, your account will be frozen on ${freezeDate}. Once frozen, you won't be able to access your data unless you accept the new pricing. Frozen accounts are permanently deleted after ${graceDays} days.`;
});
</script>

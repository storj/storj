// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        scrollable
        max-width="560px"
        transition="fade-transition"
        :persistent="loading"
    >
        <v-card ref="content">
            <v-card-item class="pa-6">
                <v-card-title class="font-weight-bold"> Add STORJ Tokens </v-card-title>
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

            <v-card-item class="py-4">
                <v-window v-model="step" :touch="false">
                    <v-window-item :value="AddTokensDialogStep.AddTokens">
                        <AddTokensStep
                            is-root
                            @close="model = false"
                            @success="() => setStep(AddTokensDialogStep.Success)"
                        />
                    </v-window-item>

                    <v-window-item :value="AddTokensDialogStep.Success">
                        <SuccessStep @continue="model = false" />
                    </v-window-item>
                </v-window>
            </v-card-item>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import { VBtn, VCard, VCardItem, VCardTitle, VDialog, VDivider, VWindow, VWindowItem } from 'vuetify/components';
import { X } from 'lucide-vue-next';

import AddTokensStep from '@/components/dialogs/upgradeAccountFlow/AddTokensStep.vue';
import SuccessStep from '@/components/dialogs/upgradeAccountFlow/SuccessStep.vue';

enum AddTokensDialogStep {
    AddTokens,
    Success,
}

const step = ref(AddTokensDialogStep.AddTokens);
const loading = ref<boolean>(false);
const content = ref<HTMLElement | null>(null);

const model = defineModel<boolean>({ required: true });

/**
 * Sets specific flow step.
 */
function setStep(s: AddTokensDialogStep) {
    step.value = s;
}

watch(content, (value) => {
    if (!value) {
        setStep(AddTokensDialogStep.AddTokens);
        return;
    }
});
</script>

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        scrollable
        width="720px"
        transition="fade-transition"
        :persistent="loading"
    >
        <v-card ref="content" rounded="xlg">
            <v-card-item class="pa-5 pl-7">
                <v-card-title class="font-weight-bold"> Add Tokens </v-card-title>
                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-card-item class="py-4">
                <v-window v-model="step">
                    <v-window-item :value="AddTokensDialogStep.AddTokens">
                        <AddTokensStep
                            is-root
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
import { computed, ref, watch } from 'vue';
import { VBtn, VCard, VCardItem, VCardTitle, VDialog, VDivider, VWindow, VWindowItem } from 'vuetify/components';

import AddTokensStep from '@poc/components/dialogs/upgradeAccountFlow/AddTokensStep.vue';
import SuccessStep from '@poc/components/dialogs/upgradeAccountFlow/SuccessStep.vue';

enum AddTokensDialogStep {
    AddTokens,
    Success,
}

const step = ref(AddTokensDialogStep.AddTokens);
const loading = ref<boolean>(false);
const content = ref<HTMLElement | null>(null);

const props = defineProps<{
    modelValue: boolean,
}>();

const emit = defineEmits<{
    'update:modelValue': [value: boolean];
}>();

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

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

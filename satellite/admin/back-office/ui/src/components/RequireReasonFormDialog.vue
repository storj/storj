// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        transition="fade-transition"
        :width
    >
        <v-card
            rounded="xlg"
            :title="step === Steps.Form ? title : 'Audit'"
            :subtitle="step === Steps.Form ? subtitle : 'Enter a reason for this change'"
        >
            <template #append>
                <v-btn
                    :icon="X" :disabled="loading"
                    variant="text" size="small" color="default" @click="model = false"
                />
            </template>

            <v-window v-model="step" :touch="false" class="pa-6" :class="{ 'no-overflow' : !overflow}">
                <v-window-item :value="Steps.Form">
                    <v-form v-model="formValid" :disabled="loading" @submit.prevent="onPrimaryAction">
                        <button type="submit" hidden />

                        <DynamicFormBuilder
                            ref="formBuilder"
                            :config="formConfig"
                            :initial-data="initialFormData"
                        />
                    </v-form>
                </v-window-item>
                <v-window-item :value="Steps.Reason">
                    <v-form :model-value="!!reason" :disabled="loading" @submit.prevent="onPrimaryAction">
                        <button type="submit" hidden />
                        <v-textarea
                            v-model="reason"
                            :rules="[RequiredRule]"
                            label="Reason"
                            placeholder="Enter reason for this change"
                            hide-details="auto"
                            variant="solo-filled"
                            autofocus
                            flat
                        />
                    </v-form>
                </v-window-item>
            </v-window>

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            :disabled="loading"
                            @click="onSecondaryAction"
                        >
                            {{ secondaryActionText }}
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary"
                            variant="flat"
                            :disabled="submitDisabled"
                            :loading="loading"
                            block
                            @click="onPrimaryAction"
                        >
                            {{ primaryActionText }}
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { VCard, VCardActions, VCol, VBtn, VDialog, VForm, VRow, VTextarea, VWindow, VWindowItem } from 'vuetify/components';
import { computed, ref, toRaw, watch } from 'vue';
import { X } from 'lucide-vue-next';

import { RequiredRule } from '@/types/common';
import { FormBuilderExpose, FormConfig } from '@/types/forms';

import DynamicFormBuilder from '@/components/form-builder/DynamicFormBuilder.vue';

enum Steps {
    Form = 1,
    Reason = 2,
}

const props = defineProps<{
    loading: boolean;
    formConfig: FormConfig;
    initialFormData: Record<string, unknown>;
    title?: string;
    subtitle?: string;
    width?: string | number;
    overflow?: boolean;
}>();

const model = defineModel<boolean>({ required: true });

const emit = defineEmits<{
    // the data contains the form data along with the reason
    (e: 'submit', data: Record<string, unknown>): void;
}>();

const step = ref<Steps>(Steps.Form);
const formBuilder = ref<FormBuilderExpose>();
const formValid = ref(false);
const reason = ref('');

const secondaryActionText = computed(() => (step.value === Steps.Form ? 'Cancel' : 'Back'));
const primaryActionText = computed(() => (step.value === Steps.Form ? 'Continue' : 'Submit'));
const submitDisabled = computed(() => (step.value === Steps.Form ? !hasFormChanged.value || !formValid.value : !reason.value));

const hasFormChanged = computed(() => {
    const formData = formBuilder.value?.getData() as Record<string, unknown> | undefined;
    if (!formData) return false;

    for (const key in props.initialFormData) {
        if (formData[key] !== props.initialFormData[key]) {
            return true;
        }
    }
    return false;
});

function onPrimaryAction() {
    if (submitDisabled.value) return;
    if (step.value === Steps.Form) {
        step.value = Steps.Reason;
    } else {
        const formData = toRaw(formBuilder.value?.getData()) as Record<string, unknown> | undefined;
        if (!formData) return;
        formData['reason'] = reason.value;
        emit('submit', formData);
    }
}

function onSecondaryAction() {
    if (step.value === Steps.Form) {
        model.value = false;
    } else {
        step.value = Steps.Form;
    }
}

watch(model, (newValue) => {
    if (!newValue) return;
    reason.value = '';
    step.value = Steps.Form;
    formBuilder.value?.reset();
});
</script>

<style scoped lang="scss">
.v-overlay .v-card .no-overflow {
    overflow-y: hidden !important;
}
</style>
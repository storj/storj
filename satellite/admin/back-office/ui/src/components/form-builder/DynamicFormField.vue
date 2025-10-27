// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <component
        :is="fieldComponent"
        :model-value="transformedValue"
        :label="field.label"
        :placeholder="field.placeholder"
        :rules="field.rules ?? []"
        :disabled="field.disabled"
        :readonly="field.readonly"
        :required="field.required"
        :clearable="field.clearable"
        :items="field.items"
        :item-title="field.itemTitle"
        :item-value="field.itemValue"
        :step="field.step"
        :precision="field.precision"
        :messages="fieldMessages"
        control-variant="stacked"
        :prepend-icon="field.prependIcon"
        :min="field.min" :max="field.max"
        :error-messages="fieldErrorMessages"
        hide-details="auto"
        variant="solo-filled"
        flat
        @update:model-value="onFieldUpdate"
    />
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VNumberInput, VSelect, VTextarea, VTextField } from 'vuetify/components';
import { VDateInput } from 'vuetify/labs/VDateInput';

import { FieldType, FormField } from '@/types/forms';

const props = defineProps<{
    field: FormField;
    value: unknown;
}>();

const emits = defineEmits<{
    (e: 'update', value: unknown): void;
}>();

const fieldComponent = computed(() => {
    switch (props.field.type) {
    case FieldType.Number:
        return VNumberInput;
    case FieldType.Select:
        return VSelect;
    case FieldType.Date:
        return VDateInput;
    case FieldType.TextArea:
        return VTextarea;
    case FieldType.Text:
    default:
        return VTextField;
    }
});

const fieldMessages = computed(() => {
    if (!props.field.messages) return undefined;

    if (typeof props.field.messages === 'function') {
        return props.field.messages(props.value);
    }

    return props.field.messages;
});

const fieldErrorMessages = computed(() => {
    if (!props.field.errorMessages) return undefined;

    if (typeof props.field.errorMessages === 'function') {
        return props.field.errorMessages(props.value);
    }

    return props.field.errorMessages;
});

/**
 * The value of the field after applying the forward transformation (if any).
 */
const transformedValue = computed(() => {
    let transformedValue = props.value;
    if (props.field.transform?.forward) {
        transformedValue = props.field.transform.forward(props.value);
    }
    return transformedValue;
});

/**
 * Handle field value updates.
 * This will reverse any transformation applied to the value
 * before emitting the update event.
 */
function onFieldUpdate(value: unknown) {
    let transformedValue = value;
    if (props.field.transform?.back) {
        transformedValue = props.field.transform.back(value);
    }
    emits('update', transformedValue);
    props.field.onUpdate?.(value);
}
</script>

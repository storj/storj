// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <template v-for="(section, sectionIndex) in config.sections" :key="sectionIndex">
        <v-divider v-if="section.divider && sectionIndex > 0" class="my-6">
            <span v-if="section.divider.text" class="text-caption text-medium-emphasis">
                {{ section.divider.text }}
            </span>
        </v-divider>

        <v-row v-for="(row, rowIndex) in section.rows" :key="`${sectionIndex}-${rowIndex}`">
            <template v-for="(field, fieldIndex) in row.fields">
                <v-col
                    v-if="!field.visible || field.visible(formData)"
                    :key="`${sectionIndex}-${rowIndex}-${fieldIndex}`"
                    :cols="field.cols?.default ?? ''"
                    :sm="field.cols?.sm"
                >
                    <DynamicFormField
                        :field="field"
                        :value="formData[field.key]"
                        @update="(value) => formData[field.key] = value"
                    />
                </v-col>
            </template>
        </v-row>
    </template>
</template>

<script setup lang="ts">
import { reactive, readonly, watch } from 'vue';
import { VCol, VDivider, VRow } from 'vuetify/components';

import { FormConfig } from '@/types/forms';

import DynamicFormField from '@/components/form-builder/DynamicFormField.vue';

const props = defineProps<{
    config: FormConfig;
    initialData: Record<string, unknown>;
}>();

const formData = reactive<Record<string, unknown>>({ ...props.initialData });

// Watch for external data changes
watch(() => props.initialData, (newData) => {
    if (!newData) return;
    Object.assign(formData, newData);
}, { deep: true });

defineExpose({
    getData: () => readonly(formData),
    reset: () => Object.assign(formData, props.initialData),
});
</script>

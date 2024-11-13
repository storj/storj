// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <p>Select default object lock mode (optional):</p>
    <v-chip-group
        v-model="defaultRetentionMode"
        filter
        selected-class="font-weight-bold"
        class="my-1"
    >
        <v-chip :value="GOVERNANCE_LOCK" variant="outlined" filter color="info">
            Governance
        </v-chip>
        <v-chip :value="COMPLIANCE_LOCK" variant="outlined" filter color="info">
            Compliance
        </v-chip>
    </v-chip-group>
    <v-alert variant="tonal" color="default">
        <p class="font-weight-bold text-body-2 mb-1 text-capitalize">Enable Object Lock ({{ defaultRetentionMode ? defaultRetentionMode.toLowerCase() : 'No Default' }} Mode)</p>
        <p class="text-subtitle-2">{{ defaultLockModeInfo }}</p>
    </v-alert>
    <template v-if="defaultRetentionMode">
        <p class="my-2">Default retention period:</p>
        <v-text-field
            v-model="period"
            variant="outlined"
            density="comfortable"
            type="number"
            :rules="rules"
            @update:model-value="updateInputText"
        >
            <template #append-inner>
                <v-menu>
                    <template #activator="{ props: slotProps, isActive }">
                        <v-btn
                            class="h-100 text-medium-emphasis"
                            variant="text"
                            density="comfortable"
                            color="default"
                            :append-icon="isActive ? ChevronUp : ChevronDown"
                            v-bind="slotProps"
                            @mousedown.stop
                            @click.stop
                        >
                            <span class="font-weight-regular">{{ periodUnit }}</span>
                        </v-btn>
                    </template>
                    <v-list v-model:selected="dropdownModel" density="compact">
                        <v-list-item :title="DefaultObjectLockPeriodUnit.DAYS" :value="DefaultObjectLockPeriodUnit.DAYS" />
                        <v-list-item :title="DefaultObjectLockPeriodUnit.YEARS" :value="DefaultObjectLockPeriodUnit.YEARS" />
                    </v-list>
                </v-menu>
            </template>
        </v-text-field>
    </template>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { VAlert, VChip, VChipGroup, VTextField, VListItem, VBtn, VMenu, VList } from 'vuetify/components';
import { ChevronDown, ChevronUp } from 'lucide-vue-next';

import { COMPLIANCE_LOCK, DefaultObjectLockPeriodUnit, GOVERNANCE_LOCK, ObjLockMode } from '@/types/objectLock';
import { RequiredRule, ValidationRule } from '@/types/common';

const emit = defineEmits<{
    'updateDefaultMode': [value: ObjLockMode | undefined];
    'updatePeriodValue': [value: number];
    'updatePeriodUnit': [value: DefaultObjectLockPeriodUnit];
}>();

const props = defineProps<{
    existingMode?: ObjLockMode;
    existingPeriod?: number;
    existingPeriodUnit?: DefaultObjectLockPeriodUnit;
}>();

const defaultRetentionMode = ref<ObjLockMode | undefined>(props.existingMode);
const period = ref<number>(props.existingPeriod ?? 0);
const periodUnit = ref<DefaultObjectLockPeriodUnit>(props.existingPeriodUnit ?? DefaultObjectLockPeriodUnit.DAYS);

const dropdownModel = computed<(DefaultObjectLockPeriodUnit.DAYS | DefaultObjectLockPeriodUnit.YEARS)[]>({
    get: () => [ periodUnit.value ],
    set: value => {
        if (value[0]) {
            periodUnit.value = value[0];
            emit('updatePeriodUnit', value[0]);
        }
    },
});

const defaultLockModeInfo = computed<string>(() => {
    if (defaultRetentionMode.value === GOVERNANCE_LOCK) {
        return 'Prevents users without special permissions from overwriting, deleting, or altering object lock settings. Users with the necessary permissions can still modify or remove locked objects.';
    }
    if (defaultRetentionMode.value === COMPLIANCE_LOCK) {
        return 'No user, including the project owner can overwrite, delete, or alter object lock settings.';
    }
    return 'Objects in this bucket will follow retention settings applied at the object level during upload or by your applications.';
});

/**
 * Returns an array of validation rules applied to the text input.
 */
const rules = computed<ValidationRule<string>[]>(() => {
    return [
        RequiredRule,
        v => !(isNaN(+v) || isNaN(parseInt(v))) || 'Invalid number',
        v => !/[.,]/.test(v) || 'Value must be a whole number',
        v => (parseInt(v) > 0) || 'Value must be a positive number',
    ];
});

function updateInputText(value: string): void {
    if (!value) {
        period.value = 0;
        return;
    }

    const num = +value;
    if (isNaN(num) || isNaN(parseInt(value))) return;
    period.value = num;
}

watch(defaultRetentionMode, (newValue) => {
    emit('updateDefaultMode', newValue);
    if (!newValue) period.value = 0;
});

watch(period, (newValue) => {
    emit('updatePeriodValue', newValue);
});
</script>

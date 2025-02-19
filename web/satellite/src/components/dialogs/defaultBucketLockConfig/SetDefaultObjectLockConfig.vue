// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <p>Select default object lock mode (optional):</p>
    <v-chip-group
        v-model="defaultRetentionMode"
        :rules="[RequiredRule]"
        filter
        mandatory
        selected-class="font-weight-bold"
        class="my-1"
    >
        <v-chip :value="NO_MODE_SET">
            No Default
        </v-chip>
        <v-chip :value="GOVERNANCE_LOCK">
            Governance
        </v-chip>
        <v-chip :value="COMPLIANCE_LOCK">
            Compliance
        </v-chip>
    </v-chip-group>
    <v-alert variant="tonal" color="info" class="mt-1">
        <p class="font-weight-bold text-body-2 mb-1 text-capitalize">Enable Object Lock ({{ defaultRetentionMode !== NO_MODE_SET ? defaultRetentionMode.toLowerCase() : 'No Default' }} Mode)</p>
        <p class="text-subtitle-2">{{ defaultLockModeInfo }}</p>
        <template v-if="defaultRetentionMode !== NO_MODE_SET">
            <p class="mt-4 mb-2 font-weight-medium">Enter the default retention period:</p>
            <v-text-field
                ref="periodInput"
                v-model="defaultRetentionPeriod"
                variant="outlined"
                density="comfortable"
                type="number"
                :rules="rules"
                hide-details="auto"
                color="primary"
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
                        <v-list v-model:selected="dropdownModel" density="compact" class="pa-1">
                            <v-list-item :title="DefaultObjectLockPeriodUnit.DAYS" :value="DefaultObjectLockPeriodUnit.DAYS" />
                            <v-list-item :title="DefaultObjectLockPeriodUnit.YEARS" :value="DefaultObjectLockPeriodUnit.YEARS" />
                        </v-list>
                    </v-menu>
                </template>
            </v-text-field>
        </template>
    </v-alert>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { VAlert, VChip, VChipGroup, VTextField, VListItem, VBtn, VMenu, VList } from 'vuetify/components';
import { ChevronDown, ChevronUp } from 'lucide-vue-next';

import {
    COMPLIANCE_LOCK,
    DefaultObjectLockPeriodUnit,
    GOVERNANCE_LOCK,
    NO_MODE_SET,
    ObjLockMode,
} from '@/types/objectLock';
import { RequiredRule, ValidationRule } from '@/types/common';

const periodInput = ref<VTextField | null>(null);

const defaultRetentionMode = defineModel<ObjLockMode | typeof NO_MODE_SET>('defaultRetentionMode', { required: true });
const defaultRetentionPeriod = defineModel<number>('defaultRetentionPeriod', { required: true });
const periodUnit = defineModel<DefaultObjectLockPeriodUnit>('periodUnit', { required: true });

const dropdownModel = computed<(DefaultObjectLockPeriodUnit.DAYS | DefaultObjectLockPeriodUnit.YEARS)[]>({
    get: () => [ periodUnit.value ],
    set: value => {
        if (value[0]) {
            periodUnit.value = value[0];
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
        v => {
            if (periodUnit.value === DefaultObjectLockPeriodUnit.DAYS && parseInt(v) > 3650)
                return 'Value must be less than or equal to 3650';
            if (periodUnit.value === DefaultObjectLockPeriodUnit.YEARS && parseInt(v) > 10)
                return 'Value must be less than or equal to 10';
            return true;
        },
    ];
});

function updateInputText(value: string): void {
    if (!value) {
        defaultRetentionPeriod.value = 0;
        return;
    }

    const num = +value;
    if (isNaN(num) || isNaN(parseInt(value))) return;
    defaultRetentionPeriod.value = num;
}

watch(defaultRetentionMode, (newValue) => {
    if (newValue === NO_MODE_SET) defaultRetentionPeriod.value = 0;
});

watch(dropdownModel, _ => {
    periodInput.value?.validate();
});
</script>

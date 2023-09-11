// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form ref="form" class="pa-8">
        <v-row>
            <v-col cols="12">
                <v-select
                    v-model="permissions"
                    :items="allPermissions"
                    label="Permissions"
                    variant="outlined"
                    color="default"
                    multiple
                    chips
                    closable-chips
                    :hide-details="false"
                    :rules="[ RequiredRule ]"
                >
                    <template #prepend-item>
                        <v-list-item
                            title="All permissions"
                            color="primary"
                            :active="areAllPermsSelected"
                            @click="toggleSelectedPerms"
                        >
                            <template #prepend>
                                <v-checkbox-btn
                                    v-model="areAllPermsSelected"
                                    :indeterminate="permissions.length != 0 && !areAllPermsSelected"
                                    color="primary"
                                />
                            </template>
                        </v-list-item>
                        <v-divider />
                    </template>

                    <template #item="{ props: slotProps }">
                        <v-list-item v-bind="slotProps" color="primary">
                            <template #prepend="{ isSelected }">
                                <v-checkbox-btn :model-value="isSelected" color="primary" />
                            </template>
                        </v-list-item>
                    </template>
                </v-select>
            </v-col>

            <v-col cols="12">
                <v-autocomplete
                    v-model="buckets"
                    v-model:search="bucketSearch"
                    class="choose-permissions-step__buckets-field"
                    :items="allBucketNames"
                    label="Buckets"
                    variant="outlined"
                    color="default"
                    no-data-text="No buckets found."
                    :placeholder="isAllBucketsSelected ? 'All buckets' : undefined"
                    :persistent-placeholder="isAllBucketsSelected"
                    multiple
                    chips
                    closable-chips
                    :hide-details="false"
                    :rules="bucketsRules"
                    :custom-filter="bucketFilter"
                >
                    <template #prepend-item>
                        <v-list-item
                            title="All buckets"
                            color="primary"
                            :active="isAllBucketsSelected"
                            @click="isAllBucketsSelected = !isAllBucketsSelected"
                        >
                            <template #prepend>
                                <v-checkbox-btn v-model="isAllBucketsSelected" color="primary" />
                            </template>
                        </v-list-item>
                        <v-divider />
                    </template>

                    <template #item="{ props: slotProps }">
                        <v-list-item v-bind="slotProps" color="primary">
                            <template #prepend="{ isSelected }">
                                <v-checkbox-btn :model-value="isSelected" color="primary" />
                            </template>
                        </v-list-item>
                    </template>
                </v-autocomplete>
            </v-col>

            <v-col cols="12">
                <v-select
                    ref="endDateSelector"
                    v-model="endDate"
                    variant="outlined"
                    color="default"
                    label="End date"
                    return-object
                    :hide-details="false"
                    :items="endDateItems"
                    :rules="[ RequiredRule ]"
                >
                    <template #append-inner>
                        <v-btn
                            class="choose-permissions-step__date-picker"
                            icon="$calendar"
                            variant="text"
                            color="default"
                            @mousedown.stop="isDatePicker = true"
                        />
                    </template>

                    <template #item="{ item, props: itemProps }">
                        <v-divider v-if="(item.raw as AccessGrantEndDate).date === null" />
                        <v-list-item v-bind="itemProps" />
                    </template>
                </v-select>
            </v-col>
        </v-row>

        <v-overlay v-model="isDatePicker" class="align-center justify-center">
            <v-date-picker
                v-model="datePickerModel"
                @click:cancel="isDatePicker = false"
                @update:model-value="onDatePickerSubmit"
            >
                <template #header />
            </v-date-picker>
        </v-overlay>
    </v-form>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
    VForm,
    VRow,
    VCol,
    VBtn,
    VDivider,
    VSelect,
    VAutocomplete,
    VListItem,
    VCheckboxBtn,
    VOverlay,
} from 'vuetify/components';
import { VDatePicker } from 'vuetify/labs/components';

import { Permission } from '@/types/createAccessGrant';
import { AccessGrantEndDate, CreateAccessStepComponent } from '@poc/types/createAccessGrant';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { SHORT_MONTHS_NAMES } from '@/utils/constants/date';
import { ValidationRule, RequiredRule } from '@poc/types/common';

type EndDateListItem = AccessGrantEndDate | { divider: true };

const allPermissions: Permission[] = [
    Permission.Read,
    Permission.Write,
    Permission.List,
    Permission.Delete,
];

const endDateItems: EndDateListItem[] = [
    { title: '1 day', date: getNowOffset(1) },
    { title: '1 week', date: getNowOffset(7) },
    { title: '1 month', date: getNowOffset(0, 1) },
    { title: '6 months', date: getNowOffset(0, 6) },
    { title: '1 year', date: getNowOffset(0, 0, 1) },
    { title: 'No end date', date: null },
];

const emit = defineEmits<{
    'permissionsChanged': [perms: Permission[]];
    'bucketsChanged': [buckets: string[]];
    'endDateChanged': [endDate: AccessGrantEndDate];
}>();

const form = ref<VForm | null>(null);
const endDateSelector = ref<VSelect | null>(null);

const permissions = ref<Permission[]>([]);

const buckets = ref<string[]>([]);
const bucketSearch = ref<string>('');
const isAllBucketsSelected = ref<boolean>(false);

const endDate = ref<AccessGrantEndDate | null>(null);
const isDatePicker = ref<boolean>(false);
const datePickerModel = ref<Date[]>([]);

watch(permissions, value => emit('permissionsChanged', value.slice()), { deep: true });
watch(buckets, value => {
    emit('bucketsChanged', value.slice());
    if (value.length) isAllBucketsSelected.value = false;
}, { deep: true });
watch(endDate, value => value && emit('endDateChanged', value));

watch(isAllBucketsSelected, value => value && (buckets.value = []));

const bucketsRules: ValidationRule<string[]>[] = [ v => (!!v.length || isAllBucketsSelected.value) || 'Required' ];

const bucketsStore = useBucketsStore();

/**
 * Indicates whether all permissions have been selected.
 */
const areAllPermsSelected = computed<boolean>(() => permissions.value.length === allPermissions.length);

/**
 * Returns all bucket names from the store.
 */
const allBucketNames = computed<string[]>(() => bucketsStore.state.allBucketNames);

/**
 * Returns whether the bucket name satisfies the query.
 */
function bucketFilter(bucketName: string, query: string): boolean {
    query = query.trim();
    if (!query) return true;

    let lastIdx = 0;
    for (const part of query.split(' ')) {
        const idx = bucketName.indexOf(part, lastIdx);
        if (idx === -1) return false;
        lastIdx = idx + part.length;
    }
    return true;
}

/**
 * Selects or deselects all permissions.
 */
function toggleSelectedPerms(): void {
    if (permissions.value.length !== allPermissions.length) {
        permissions.value = allPermissions.slice();
        return;
    }
    permissions.value = [];
}

/**
 * Returns the current date offset by the specified amount.
 */
function getNowOffset(days = 0, months = 0, years = 0): Date {
    const now = new Date();
    return new Date(
        now.getFullYear() + years,
        now.getMonth() + months,
        now.getDate() + days,
        11, 59, 59,
    );
}

/**
 * Stores the access grant end date from the date picker.
 */
function onDatePickerSubmit(): void {
    if (!datePickerModel.value.length) return;

    const date = datePickerModel.value[0];
    endDate.value = {
        title: `${date.getDate()} ${SHORT_MONTHS_NAMES[date.getMonth()]} ${date.getFullYear()}`,
        date: new Date(date.getFullYear(), date.getMonth(), date.getDate(), 11, 59, 59),
    };

    isDatePicker.value = false;
}

defineExpose<CreateAccessStepComponent>({
    title: 'Access Permissions',
    validate: () => {
        form.value?.validate();
        return !!form.value?.isValid;
    },
});
</script>

<style scoped lang="scss">
.v-field {

    &:not(.v-field--error) .choose-permissions-step__date-picker :deep(.v-icon) {
        opacity: var(--v-medium-emphasis-opacity);
    }

    &.v-field--error .choose-permissions-step__date-picker {
        color: rgb(var(--v-theme-error));
    }
}

.choose-permissions-step__buckets-field :deep(input::placeholder) {
    opacity: 1;
}
</style>

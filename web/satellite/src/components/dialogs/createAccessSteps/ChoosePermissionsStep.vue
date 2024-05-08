// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form ref="form" class="pa-6">
        <v-row>
            <v-col cols="12">
                <p class="text-subtitle-2 font-weight-bold mb-5">Select the permissions for this access</p>

                <v-select
                    v-model="permissions"
                    :items="allPermissions"
                    label="Access Permissions"
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
                            :active="areAllPermsSelected"
                            density="compact"
                            @click="toggleSelectedPerms"
                        >
                            <template #prepend>
                                <v-checkbox-btn
                                    v-model="areAllPermsSelected"
                                    :indeterminate="permissions.length != 0 && !areAllPermsSelected"
                                />
                            </template>
                        </v-list-item>
                        <v-divider />
                    </template>

                    <template #item="{ props: slotProps }">
                        <v-list-item v-bind="slotProps" density="compact">
                            <template #prepend="{ isSelected }">
                                <v-checkbox-btn :model-value="isSelected" />
                            </template>
                        </v-list-item>
                    </template>
                </v-select>

                <!-- <v-chip-group
                    v-model="permissions"
                    column
                    multiple
                    class="mb-3"
                >
                    <v-chip
                        rounded="xl"
                        class="text-body-2"
                        filter
                        variant="outlined"
                        value="All"
                    >
                        All
                    </v-chip>
                    <v-chip
                        v-for="perm in allPermissions"
                        :key="perm"
                        :value="perm"
                        rounded="xl"
                        class="text-body-2"
                        filter
                        variant="outlined"
                    >
                        {{ perm }}
                    </v-chip>
                </v-chip-group> -->
            </v-col>

            <v-col cols="12">
                <p class="text-subtitle-2 font-weight-bold mb-5">Choose buckets to give access to</p>

                <v-autocomplete
                    v-model="buckets"
                    v-model:search="bucketSearch"
                    class="choose-permissions-step__buckets-field"
                    :items="allBucketNames"
                    label="Access Buckets"
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
                            :active="isAllBucketsSelected"
                            density="compact"
                            @click="isAllBucketsSelected = !isAllBucketsSelected"
                        >
                            <template #prepend>
                                <v-checkbox-btn v-model="isAllBucketsSelected" />
                            </template>
                        </v-list-item>
                        <v-divider />
                    </template>

                    <template #item="{ props: slotProps }">
                        <v-list-item v-bind="slotProps" density="compact">
                            <template #prepend="{ isSelected }">
                                <v-checkbox-btn :model-value="isSelected" />
                            </template>
                        </v-list-item>
                    </template>
                </v-autocomplete>

                <!-- <v-chip-group
                    v-model="buckets"
                    column
                    multiple
                    class="mb-3"
                >
                    <v-chip
                        class="text-body-2"
                        rounded="xl"
                        filter
                        variant="outlined"
                        value="All Buckets"
                    >
                        All Buckets
                    </v-chip>
                </v-chip-group> -->
            </v-col>

            <v-col cols="12">
                <p class="text-subtitle-2 font-weight-bold mb-5">Choose when your access will expire</p>

                <v-select
                    ref="endDateSelector"
                    v-model="endDate"
                    variant="outlined"
                    color="default"
                    label="Access Expiration Date"
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
                        <v-list-item v-bind="itemProps" density="compact" />
                    </template>
                </v-select>

                <!-- <v-chip-group
                    v-model="endDate"
                    column
                >
                    <v-chip
                        class="text-body-2"
                        rounded="xl"
                        filter
                        variant="outlined"
                        value="No end date"
                    >
                        No expiration
                    </v-chip>
                </v-chip-group> -->
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
    VDatePicker,
} from 'vuetify/components';

import { AccessGrantEndDate, Permission } from '@/types/createAccessGrant';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { SHORT_MONTHS_NAMES } from '@/utils/constants/date';
import { ValidationRule, RequiredRule, DialogStepComponent } from '@/types/common';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';

type EndDateListItem = AccessGrantEndDate | { divider: true };

const notify = useNotify();

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
    { title: 'No expiration', date: null },
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
const datePickerModel = ref<Date>();

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
    if (!datePickerModel.value) return;

    const date = datePickerModel.value;
    const submitted = new Date(
        date.getFullYear(),
        date.getMonth(),
        date.getDate(),
        11, 59, 59,
    );

    if (submitted.getTime() < new Date().getTime()) {
        notify.error('Please select future date', AnalyticsErrorEventSource.CREATE_AG_MODAL);
        return;
    }

    endDate.value = {
        title: `${date.getDate()} ${SHORT_MONTHS_NAMES[date.getMonth()]} ${date.getFullYear()}`,
        date: submitted,
    };

    isDatePicker.value = false;
}

defineExpose<DialogStepComponent>({
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

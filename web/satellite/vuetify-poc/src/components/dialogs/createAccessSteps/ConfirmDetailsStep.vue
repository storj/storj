// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pa-8">
        <v-row>
            <v-col cols="12">
                Confirm that the access details are correct before creating.
            </v-col>
            <v-divider class="my-1" />
            <v-col class="pa-1" cols="12">
                <div
                    v-for="item in items"
                    :key="item.title"
                    class="d-flex justify-space-between ma-2"
                >
                    <p class="flex-shrink-0 mr-4">{{ item.title }}</p>
                    <p class="text-body-2 text-medium-emphasis text-right">{{ item.value }}</p>
                </div>
            </v-col>
        </v-row>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VRow, VCol, VDivider } from 'vuetify/components';

import { Permission, AccessType } from '@/types/createAccessGrant';
import { AccessGrantEndDate, CreateAccessStepComponent } from '@poc/types/createAccessGrant';

interface Item {
    title: string;
    value: string;
}

const props = defineProps<{
    name: string;
    types: AccessType[];
    permissions: Permission[];
    buckets: string[];
    endDate: AccessGrantEndDate;
}>();

/**
 * Returns the data used to generate the info rows.
 */
const items = computed<Item[]>(() => {
    return [
        { title: 'Name', value: props.name },
        { title: 'Type', value: props.types.join(', ') },
        { title: 'Permissions', value: props.permissions.join(', ') },
        { title: 'Buckets', value: props.buckets.join(', ') || 'All buckets' },
        { title: 'End Date', value: props.endDate.title },
    ];
});

defineExpose<CreateAccessStepComponent>({
    title: 'Confirm Details',
});
</script>

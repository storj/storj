// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pa-6">
        <v-row>
            <v-col cols="12">
                <p>Confirm that the access details are correct before creating.</p>
                <v-list lines="one">
                    <v-list-item
                        v-for="item in items"
                        :key="item.title"
                        :title="item.title"
                        :subtitle="item.value"
                        class="pl-0"
                    />
                </v-list>
            </v-col>
        </v-row>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VRow, VCol, VList, VListItem } from 'vuetify/components';

import { Permission, AccessType, ObjectLockPermission } from '@/types/setupAccess';

interface Item {
    title: string;
    value: string;
}

const props = defineProps<{
    name: string;
    type: AccessType;
    permissions: Permission[];
    objectLockPermissions: ObjectLockPermission[];
    buckets: string[];
    endDate: Date | null;
}>();

/**
 * Returns the data used to generate the info rows.
 */
const items = computed<Item[]>(() => {
    const its = [
        { title: 'Name', value: props.name },
        { title: 'Type', value: props.type },
        { title: 'Permissions', value: props.permissions.join(', ') },
        { title: 'Buckets', value: props.buckets.join(', ') || 'All buckets' },
        { title: 'Expiration Date', value: props.endDate ? props.endDate.toLocaleString() : 'No expiration date' },
    ];

    if (props.objectLockPermissions.length) {
        its.splice(2, 0, { title: 'Object Lock Permissions', value: props.objectLockPermissions.join(', ') });
    }

    return its;
});
</script>

<style scoped>
:deep(.v-list-item-subtitle) {
    -webkit-line-clamp: unset !important;
    white-space: normal !important;
    white-space-collapse: collapse !important;
    text-wrap: wrap !important;
}
</style>

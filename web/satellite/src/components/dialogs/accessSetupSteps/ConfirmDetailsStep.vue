// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pa-6">
        <v-row>
            <v-col cols="12">
                <p>Confirm that the access details are correct before creating.</p>
                <v-list>
                    <v-list-item
                        v-for="item in items"
                        :key="item.title"
                        :title="item.title"
                        :subtitle="item.value"
                        class="mb-2 pl-0"
                    >
                        <template #title>
                            <p class="text-medium-emphasis text-body-2 mb-1">{{ item.title }}</p>
                        </template>
                        <template #subtitle>
                            <p v-if="item.title === 'Object Lock Permissions'" class="text-body-2">
                                <span v-if="hasBypass">
                                    <v-tooltip width="300" activator="parent">
                                        Warning: <b><i>BypassGovernanceRetention</i></b> allows users to delete or
                                        modify objects even when under retention policies. Only grant
                                        this permission when necessary, as it may lead to premature
                                        data deletion or compliance issues.
                                    </v-tooltip>
                                    ⚠️ {{ ObjectLockPermission.BypassGovernanceRetention }}
                                </span>
                                <!-- Add a comma if there are multiple object lock permissions -->
                                <span v-if="hasBypass && objectLockPermissions.length > 1">,</span>
                                {{ item.value }}
                            </p>
                            <p v-else class="text-body-2">{{ item.value }}</p>
                        </template>
                    </v-list-item>
                </v-list>
            </v-col>
        </v-row>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VCol, VList, VListItem, VRow, VTooltip } from 'vuetify/components';

import { AccessType, ObjectLockPermission, Permission } from '@/types/setupAccess';

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

    if (!props.objectLockPermissions.length) {
        return its;
    }

    const lockPermissions = props.objectLockPermissions.filter(p => p !== ObjectLockPermission.BypassGovernanceRetention);
    its.splice(2, 0, { title: 'Object Lock Permissions', value: lockPermissions.join(', ') });

    return its;
});

const hasBypass = computed(() => props.objectLockPermissions.includes(ObjectLockPermission.BypassGovernanceRetention));
</script>

<style scoped>
:deep(.v-list-item-subtitle) {
    -webkit-line-clamp: unset !important;
    white-space: normal !important;
    white-space-collapse: collapse !important;
    text-wrap: wrap !important;
}
</style>
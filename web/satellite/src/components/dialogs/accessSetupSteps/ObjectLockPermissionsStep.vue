// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-6" @submit.prevent>
        <v-row>
            <v-col cols="12">
                <p>Select the object lock permissions you want to allow.</p>
                <v-btn
                    :color="permissions.length === allPermissions.length ? 'info' : 'secondary'"
                    variant="outlined"
                    density="compact"
                    size="default"
                    class="mt-4 text-body-2"
                    rounded="md"
                    @click="onAllClick"
                >
                    <template v-if="permissions.length === allPermissions.length" #prepend>
                        <v-icon><Check :stroke-width="4" /></v-icon>
                    </template>
                    All Permissions
                </v-btn>
                <v-chip-group
                    v-model="permissions"
                    variant="outlined"
                    filter
                    column
                    multiple
                    selected-class="text-info font-weight-bold"
                    class="mt-2 mb-3"
                >
                    <v-chip
                        v-for="permission in allPermissions"
                        :key="permission"
                        :value="permission"
                        filter
                    >
                        {{ permission }}
                    </v-chip>
                </v-chip-group>

                <v-expansion-panels static>
                    <v-expansion-panel
                        title="Permissions Information"
                        elevation="0"
                        rounded="lg"
                        class="border my-4 font-weight-bold"
                        static
                    >
                        <v-expansion-panel-text class="text-body-2">
                            <p class="my-2"><span class="font-weight-bold">PutObjectRetention</span>: Allows you to set retention policies, protecting objects from deletion or modification until the retention period expires.</p>
                            <p class="my-2"><span class="font-weight-bold">GetObjectRetention</span>: Allows you to view the retention settings of objects, helping ensure compliance with retention policies.</p>
                            <p class="my-2"><span class="font-weight-bold">BypassGovernanceRetention</span>: Allows you to bypass governance-mode retention, enabling deletion of objects before the retention period ends.</p>
                            <p class="my-2"><span class="font-weight-bold">PutObjectLegalHold</span>: Allows you to place a legal hold on objects, preventing deletion or modification regardless of retention policies.</p>
                            <p class="my-2"><span class="font-weight-bold">GetObjectLegalHold</span>: Allows you to view the legal hold status of objects, which is useful for auditing and compliance purposes.</p>
                        </v-expansion-panel-text>
                    </v-expansion-panel>
                </v-expansion-panels>
            </v-col>
        </v-row>
    </v-form>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import {
    VChip,
    VChipGroup,
    VCol,
    VForm,
    VRow,
    VBtn,
    VIcon,
    VExpansionPanels,
    VExpansionPanel,
    VExpansionPanelText,
} from 'vuetify/components';
import { Check } from 'lucide-vue-next';

import { ObjectLockPermission } from '@/types/setupAccess';

const emit = defineEmits<{
    'permissionsChanged': [perms: ObjectLockPermission[]];
}>();

const permissions = ref<ObjectLockPermission[]>([]);

const allPermissions = [
    ObjectLockPermission.PutObjectRetention,
    ObjectLockPermission.GetObjectRetention,
    ObjectLockPermission.BypassGovernanceRetention,
    ObjectLockPermission.PutObjectLegalHold,
    ObjectLockPermission.GetObjectLegalHold,
];

/**
 * Selects or deselects all the permissions.
 */
function onAllClick(): void {
    permissions.value = permissions.value.length === allPermissions.length ? [] : allPermissions;
}

watch(permissions, value => {
    emit('permissionsChanged', value.slice());
}, { deep: true });
</script>

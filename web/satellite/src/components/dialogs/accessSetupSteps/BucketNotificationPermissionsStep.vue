// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-6" @submit.prevent>
        <v-row>
            <v-col cols="12">
                <p>Select the bucket notification permissions you want to allow.</p>
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
                        <v-icon><Check /></v-icon>
                    </template>
                    All Permissions
                </v-btn>
                <v-chip-group
                    v-model="permissions"
                    variant="outlined"
                    filter
                    column
                    multiple
                    selected-class="font-weight-bold"
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
                        <v-expansion-panel-text class="text-body-2 overflow-y-auto">
                            <p class="my-2"><span class="font-weight-bold">PutBucketNotificationConfiguration</span>: Allows you to configure bucket event notifications, enabling you to receive alerts when objects are created, deleted, or modified in the bucket.</p>
                            <p class="my-2"><span class="font-weight-bold">GetBucketNotificationConfiguration</span>: Allows you to retrieve the current notification configuration for a bucket, so you can view which events are being monitored.</p>
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
    VBtn,
    VChip,
    VChipGroup,
    VCol,
    VExpansionPanel,
    VExpansionPanelText,
    VExpansionPanels,
    VForm,
    VIcon,
    VRow,
} from 'vuetify/components';
import { Check } from 'lucide-vue-next';

import { BucketNotificationPermission } from '@/types/setupAccess';

const emit = defineEmits<{
    'permissionsChanged': [perms: BucketNotificationPermission[]];
}>();

const permissions = ref<BucketNotificationPermission[]>([]);

const allPermissions = [
    BucketNotificationPermission.PutBucketNotificationConfiguration,
    BucketNotificationPermission.GetBucketNotificationConfiguration,
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

<style scoped lang="scss">
:deep(.v-expansion-panel-text__wrapper) {
    height: 25vh;
}
</style>

// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-6" @submit.prevent>
        <v-row>
            <v-col cols="12">
                <p class="font-weight-bold mb-2">
                    Choose Permissions
                </p>
                <p>Select which permissions to give this application.</p>
                <v-btn
                    :color="permissions.length === 4 ? 'success' : 'default'"
                    density="compact"
                    variant="outlined"
                    size="default"
                    :prepend-icon="permissions.length === 4 ? mdiCheckCircle : undefined"
                    class="mt-3"
                    @click="onAllClick"
                >
                    All
                </v-btn>
                <v-chip-group
                    v-model="permissions"
                    filter
                    multiple
                    selected-class="text-primary font-weight-bold"
                    class="my-2"
                >
                    <v-chip
                        :key="Permission.Read"
                        variant="outlined"
                        filter
                        :value="Permission.Read"
                        color="secondary"
                    >
                        Read
                    </v-chip>

                    <v-chip
                        :key="Permission.Write"
                        variant="outlined"
                        filter
                        :value="Permission.Write"
                        color="secondary"
                    >
                        Write
                    </v-chip>

                    <v-chip
                        :key="Permission.List"
                        variant="outlined"
                        filter
                        :value="Permission.List"
                        color="secondary"
                    >
                        List
                    </v-chip>

                    <v-chip
                        :key="Permission.Delete"
                        variant="outlined"
                        filter
                        :value="Permission.Delete"
                        color="secondary"
                    >
                        Delete
                    </v-chip>
                </v-chip-group>
                <v-alert variant="tonal" color="info" width="auto">
                    <p class="text-subtitle-2 font-weight-bold">
                        Important
                    </p>
                    <p class="text-subtitle-2">
                        If you don't select the correct permissions, your application might not connect properly.
                    </p>
                </v-alert>
            </v-col>
        </v-row>
    </v-form>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import { VAlert, VChip, VChipGroup, VCol, VForm, VRow, VBtn } from 'vuetify/components';
import { mdiCheckCircle } from '@mdi/js';

import { Permission } from '@/types/createAccessGrant';

const emit = defineEmits<{
    'permissionsChanged': [perms: Permission[]];
}>();

const permissions = ref<Permission[]>([]);

/**
 * Selects or deselects all the permissions.
 */
function onAllClick(): void {
    permissions.value = permissions.value.length === 4 ?
        [] :
        [
            Permission.Read,
            Permission.Write,
            Permission.List,
            Permission.Delete,
        ];
}

watch(permissions, value => emit('permissionsChanged', value.slice()), { deep: true });
</script>

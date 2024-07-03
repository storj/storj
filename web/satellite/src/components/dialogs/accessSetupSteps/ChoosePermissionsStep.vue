// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-6" @submit.prevent>
        <v-row>
            <v-col cols="12">
                <p>Select the permissions you want to allow.</p>
                <v-btn
                    :color="permissions.length === 4 ? 'info' : 'secondary'"
                    variant="outlined"
                    density="compact"
                    size="default"
                    :prepend-icon="permissions.length === 4 ? mdiCheckBold : undefined"
                    class="mt-4 text-body-2"
                    rounded="md"
                    @click="onAllClick"
                >
                    All Permissions
                </v-btn>
                <v-chip-group
                    v-model="permissions"
                    variant="outlined"
                    filter
                    multiple
                    selected-class="text-info font-weight-bold"
                    class="mt-2 mb-3"
                >
                    <v-chip
                        :key="Permission.Read"
                        filter
                        :value="Permission.Read"
                    >
                        Read
                    </v-chip>

                    <v-chip
                        :key="Permission.Write"
                        filter
                        :value="Permission.Write"
                    >
                        Write
                    </v-chip>

                    <v-chip
                        :key="Permission.List"
                        filter
                        :value="Permission.List"
                    >
                        List
                    </v-chip>

                    <v-chip
                        :key="Permission.Delete"
                        filter
                        :value="Permission.Delete"
                    >
                        Delete
                    </v-chip>
                </v-chip-group>
                <v-alert variant="tonal" width="auto">
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
import { mdiCheckBold } from '@mdi/js';

import { Permission } from '@/types/setupAccess';

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

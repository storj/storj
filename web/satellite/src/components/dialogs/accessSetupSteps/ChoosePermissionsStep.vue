// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-6" @submit.prevent>
        <v-row>
            <v-col cols="12">
                <p>Select the permissions you want to allow.</p>
                <v-btn
                    :color="permissions.length === 4 ? 'primary' : 'secondary'"
                    variant="outlined"
                    density="compact"
                    size="default"
                    class="mt-4 text-body-2"
                    rounded="md"
                    @click="onAllClick"
                >
                    <template v-if="permissions.length === 4" #prepend>
                        <v-icon><Check /></v-icon>
                    </template>
                    All Permissions
                </v-btn>
                <v-chip-group
                    v-model="permissions"
                    variant="outlined"
                    filter
                    multiple
                    selected-class="font-weight-bold"
                    class="mt-2"
                    :class="{ 'mb-3': !invalid }"
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
                <span v-if="invalid" class="text-caption d-block text-error mb-3">No permission selected</span>
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
import { VAlert, VChip, VChipGroup, VCol, VForm, VRow, VBtn, VIcon } from 'vuetify/components';
import { Check } from 'lucide-vue-next';

import { Permission } from '@/types/setupAccess';

const emit = defineEmits<{
    'permissionsChanged': [perms: Permission[]];
}>();

const invalid = ref<boolean>(false);
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

watch(permissions, value => {
    invalid.value = value.length === 0;
    emit('permissionsChanged', value.slice());
}, { deep: true });

defineExpose({
    validate: () => {
        invalid.value = permissions.value.length === 0;
        return !invalid.value;
    },
});
</script>

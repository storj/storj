// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-menu activator="parent">
        <v-list class="pa-2">
            <v-list-item
                v-if="!license.revokedAt && !isExpired"
                density="comfortable" link
                rounded="lg" base-color="warning"
                @click="emit('revoke', license)"
            >
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Revoke
                </v-list-item-title>
            </v-list-item>

            <v-list-item
                density="comfortable" link
                rounded="lg" base-color="error"
                @click="emit('delete', license)"
            >
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Delete
                </v-list-item-title>
            </v-list-item>
        </v-list>
    </v-menu>
</template>

<script setup lang="ts">
import { VMenu, VList, VListItem, VListItemTitle } from 'vuetify/components';
import { computed } from 'vue';

import { UserLicense } from '@/api/client.gen';

const props = defineProps<{
    license: UserLicense;
}>();

const emit = defineEmits<{
    (e: 'revoke', license: UserLicense): void;
    (e: 'delete', license: UserLicense): void;
}>();

const isExpired = computed(() => {
    if (!props.license.expiresAt) return false;
    return new Date(props.license.expiresAt) < new Date();
});
</script>

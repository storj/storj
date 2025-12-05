// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-3" @submit.prevent>
        <v-card-text>
            In your DNS provider, create 3 TXT records.
            <v-text-field variant="solo-filled" flat class="my-4" label="TXT Hostname" :model-value="`txt-${domain}`" readonly hide-details>
                <template #append-inner>
                    <input-copy-button :value="`txt-${domain}`" />
                </template>
            </v-text-field>
            <v-text-field variant="solo-filled" flat class="my-4" label="TXT1 Content" :model-value="storjRoot" readonly hide-details>
                <template #append-inner>
                    <input-copy-button :value="storjRoot" />
                </template>
            </v-text-field>
            <v-textarea variant="solo-filled" flat class="my-4" rows="2" auto-grow no-resize label="TXT2 Content" :model-value="storjAccess" readonly hide-details>
                <template #append-inner>
                    <input-copy-button :value="storjAccess" />
                </template>
            </v-textarea>
            <v-text-field v-if="isPaidTier" variant="solo-filled" flat class="my-4" label="TXT3 Content" :model-value="storjTls" readonly hide-details>
                <template #append-inner>
                    <input-copy-button :value="storjTls" />
                </template>
            </v-text-field>
        </v-card-text>
    </v-form>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VCardText, VForm, VTextField, VTextarea } from 'vuetify/components';

import { useUsersStore } from '@/store/modules/usersStore';

import InputCopyButton from '@/components/InputCopyButton.vue';

defineProps<{
    domain: string
    storjRoot: string
    storjAccess: string
    storjTls: string
}>();

const usersStore = useUsersStore();

const isPaidTier = computed<boolean>(() => usersStore.state.user.hasPaidPrivileges);
</script>

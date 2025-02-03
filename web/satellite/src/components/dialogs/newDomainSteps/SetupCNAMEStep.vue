// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-3" @submit.prevent>
        <v-card-text>
            In your DNS provider, create CNAME record.
            <v-text-field variant="solo-filled" flat class="my-4" label="Hostname" :model-value="domain" readonly hide-details>
                <template #append-inner>
                    <input-copy-button :value="domain" />
                </template>
            </v-text-field>
            <v-text-field variant="solo-filled" flat class="my-4" label="Content" :model-value="cname" readonly hide-details>
                <template #append-inner>
                    <input-copy-button :value="cname" />
                </template>
            </v-text-field>
            <v-alert type="info" variant="tonal" class="mb-4">Ensure you include the dot . at the end</v-alert>
            The next step is creating the TXT records.
        </v-card-text>
    </v-form>
</template>

<script setup lang="ts">
import { VAlert, VCardText, VForm, VTextField } from 'vuetify/components';

import InputCopyButton from '@/components/InputCopyButton.vue';

defineProps<{
    domain: string
    cname: string
}>();
</script>

// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-3" @submit.prevent>
        <v-card-text>
            In your DNS provider, create 3 TXT records.
            <v-text-field variant="solo-filled" flat class="my-4" density="comfortable" label="TXT Hostname" :model-value="`txt-${domain}`" readonly hide-details />
            <v-text-field variant="solo-filled" flat class="my-4" density="comfortable" label="TXT1 Content" :model-value="`storj-root:${bucket}`" readonly hide-details />
            <v-textarea variant="solo-filled" flat class="my-4" density="comfortable" rows="2" auto-grow no-resize label="TXT2 Content" :model-value="`storj-access:${access}`" readonly hide-details />
            <v-text-field variant="solo-filled" flat class="my-4" density="comfortable" label="TXT3 Content" model-value="storj-tls:true" readonly hide-details />
        </v-card-text>
    </v-form>
</template>

<script setup lang="ts">
import { VCardText, VForm, VTextField, VTextarea } from 'vuetify/components';

defineProps<{
    domain: string
    bucket: string | undefined
    access: string
}>();
</script>

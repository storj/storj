// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-3" @submit.prevent>
        <v-card-text>
            In your DNS provider, create CNAME record.
            <v-text-field variant="solo-filled" flat class="my-4" density="comfortable" label="Hostname" :model-value="domain" readonly hide-details />
            <v-text-field variant="solo-filled" flat class="my-4" density="comfortable" label="Content" :model-value="content" readonly hide-details />
            <v-alert type="info" variant="tonal" class="mb-4">Ensure you include the dot . at the end</v-alert>
            The next step is creating the TXT records.
        </v-card-text>
    </v-form>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VAlert, VCardText, VForm, VTextField } from 'vuetify/components';

import { useLinksharing } from '@/composables/useLinksharing';

defineProps<{
    domain: string
}>();

const { publicLinksharingURL } = useLinksharing();

const content = computed<string>(() => `${publicLinksharingURL.value.split('//').pop() ?? ''}.`);
</script>

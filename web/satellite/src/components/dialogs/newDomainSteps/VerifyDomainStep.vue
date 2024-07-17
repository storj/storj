// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-3" @submit.prevent>
        <v-card-text>
            Check to make sure your DNS records are ready.
            <v-btn block variant="tonal" class="mt-3" :loading="isLoading" @click="checkDNSRecord">Check DNS</v-btn>
        </v-card-text>
    </v-form>
</template>

<script setup lang="ts">
import { VBtn, VCardText, VForm } from 'vuetify/components';

import { useDomainsStore } from '@/store/modules/domainsStore';
import { useNotify } from '@/utils/hooks';
import { useLoading } from '@/composables/useLoading';

const props = defineProps<{
    domain: string
}>();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const domainsStore = useDomainsStore();

function checkDNSRecord(): void {
    withLoading(async () => {
        try {
            await domainsStore.checkDNSRecord(props.domain);
        } catch (error) {
            notify.error(error.message);
        }
    });
}
</script>

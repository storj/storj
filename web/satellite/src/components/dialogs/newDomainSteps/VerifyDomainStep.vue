// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-3" @submit.prevent>
        <v-card-text>
            Check to make sure your DNS records are ready.
            <v-btn
                block
                variant="tonal"
                :color="isSuccess ? 'success' : 'primary'"
                class="mt-3"
                :loading="isLoading"
                @click="checkDNSRecords"
            >
                {{ isSuccess ? 'Success' : 'Check DNS' }}
            </v-btn>
        </v-card-text>
    </v-form>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { VBtn, VCardText, VForm } from 'vuetify/components';

import { useDomainsStore } from '@/store/modules/domainsStore';
import { useNotify } from '@/utils/hooks';
import { useLoading } from '@/composables/useLoading';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

const props = defineProps<{
    domain: string
    cname: string
    txt: string[]
}>();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const domainsStore = useDomainsStore();

const isSuccess = ref<boolean>(false);

function checkDNSRecords(): void {
    if (isSuccess.value) return;

    withLoading(async () => {
        try {
            await domainsStore.checkDNSRecords(props.domain, props.cname, props.txt);
            isSuccess.value = true;

            setTimeout(() => {
                isSuccess.value = false;
            }, 3000);
        } catch (error) {
            notify.error(error.message, AnalyticsErrorEventSource.NEW_DOMAIN_MODAL);
        }
    });
}
</script>

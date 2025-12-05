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
            <v-alert
                v-if="!isSuccess && isVerifyError"
                color="error"
                variant="tonal"
                class="mt-4"
                title="Unable to verify domain"
                text="DNS record not found. Please check your DNS configuration."
            />
            <v-alert
                v-if="!isSuccess && isNotCorrectError"
                color="error"
                variant="tonal"
                class="mt-4"
                :title="`${notCorrectRecordType} is not correct`"
                :text="`Please update the following ${notCorrectRecordType} record:`"
            >
                <v-textarea :rows="notCorrectRecordType === 'TXT' ? 3 : 1" variant="solo-filled" flat class="mt-4" density="comfortable" label="Incorrect" :model-value="got" readonly hide-details />
                <v-textarea :rows="notCorrectRecordType === 'TXT' ? 3 : 1" variant="solo-filled" flat class="mt-4" density="comfortable" label="Correct" :model-value="expected" readonly hide-details />
            </v-alert>
        </v-card-text>
    </v-form>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { VBtn, VCardText, VForm, VAlert, VTextarea } from 'vuetify/components';

import { useDomainsStore } from '@/store/modules/domainsStore';
import { useNotify } from '@/composables/useNotify';
import { useLoading } from '@/composables/useLoading';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { CheckDNSResponse } from '@/types/domains';

const props = defineProps<{
    domain: string
    cname: string
    txt: string[]
}>();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const domainsStore = useDomainsStore();

const isSuccess = ref<boolean>(false);
const isVerifyError = ref<boolean>(false);
const isNotCorrectError = ref<boolean>(false);
const notCorrectRecordType = ref<'TXT' | 'CNAME'>('CNAME');
const expected = ref<string>('');
const got = ref<string>('');

function checkDNSRecords(): void {
    if (isSuccess.value) return;

    withLoading(async () => {
        try {
            const response: CheckDNSResponse = await domainsStore.checkDNSRecords(props.domain, props.cname, props.txt);
            switch (true) {
            case response.isSuccess:
                if (isVerifyError.value) isVerifyError.value = false;
                if (isNotCorrectError.value) isNotCorrectError.value = false;

                isSuccess.value = true;
                setTimeout(() => {
                    isSuccess.value = false;
                }, 3000);

                return;
            case response.isVerifyError:
                isVerifyError.value = true;

                return;
            case response.expectedCNAME !== '' && response.gotCNAME !== '':
                expected.value = response.expectedCNAME;
                got.value = response.gotCNAME;
                notCorrectRecordType.value = 'CNAME';
                isNotCorrectError.value = true;

                return;
            case response.expectedTXT.length > 0 && response.gotTXT.length > 0:
                expected.value = response.expectedTXT.join('\n\n');
                got.value = response.gotTXT.join('\n\n');
                notCorrectRecordType.value = 'TXT';
                isNotCorrectError.value = true;

                return;
            default:
                notify.error('Cannot check DNS records', AnalyticsErrorEventSource.NEW_DOMAIN_MODAL);
            }
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.NEW_DOMAIN_MODAL);
        }
    });
}
</script>

// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" width="600" transition="fade-transition">
        <v-card
            rounded="xlg"
            title="Update License"
            subtitle="Update license expiration time"
        >
            <template #append>
                <v-btn
                    :icon="X" :disabled="isLoading"
                    variant="text" size="small" color="default" @click="model = false"
                />
            </template>

            <div class="pa-6">
                <v-row>
                    <v-col cols="12">
                        <v-alert type="info" variant="tonal" density="compact" class="mb-2">
                            Only the expiration date can be changed. All other license fields remain unchanged.
                        </v-alert>
                    </v-col>
                    <v-col v-if="errorMessage" cols="12">
                        <v-alert type="error" variant="tonal" density="compact" class="mb-2">
                            {{ errorMessage }}
                        </v-alert>
                    </v-col>
                    <v-col v-if="license" cols="12">
                        <p class="text-body-2 mb-1"><strong>License Type:</strong> {{ licenseTypeLabel(license.type) }}</p>
                        <p class="text-body-2 mb-1"><strong>Project:</strong> {{ license.publicId || 'All Projects' }}</p>
                        <p class="text-body-2 mb-1"><strong>Bucket:</strong> {{ license.bucketName || 'All Buckets' }}</p>
                        <p class="text-body-2 mb-1"><strong>Current Expiration:</strong> {{ date.format(license.expiresAt, 'fullDateTime') }}</p>
                    </v-col>
                    <v-col cols="12">
                        <v-date-input
                            v-model="newExpiresAt"
                            label="New Expiration Date"
                            :min="minDate"
                            :rules="[RequiredRule]"
                            variant="solo-filled"
                            hide-details="auto"
                            prepend-icon=""
                            prepend-inner-icon="$calendar"
                        />
                    </v-col>
                    <v-col cols="12">
                        <v-textarea
                            v-model="reason"
                            :rules="[RequiredRule]"
                            label="Reason"
                            placeholder="Enter reason for updating this license"
                            variant="solo-filled"
                            hide-details="auto"
                            flat
                        />
                    </v-col>
                </v-row>
            </div>

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block @click="model = false">Cancel</v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            variant="flat"
                            color="primary"
                            :loading="isLoading"
                            :disabled="!reason || !newExpiresAt"
                            block
                            @click="updateLicense"
                        >
                            Update License
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { VAlert, VBtn, VCard, VCardActions, VCol, VDialog, VRow, VTextarea } from 'vuetify/components';
import { VDateInput } from 'vuetify/labs/VDateInput';
import { X } from 'lucide-vue-next';
import { ref, watch } from 'vue';
import { useDate } from 'vuetify';

import { UserLicense } from '@/api/client.gen';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { useUsersStore } from '@/store/users';
import { RequiredRule } from '@/types/common';
import { licenseTypeLabel } from '@/utils/licenses';

const notify = useNotify();
const usersStore = useUsersStore();
const { isLoading, withLoading } = useLoading();
const date = useDate();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    userId: string;
    license: UserLicense | null;
}>();

const emit = defineEmits<{
    success: [];
}>();

const newExpiresAt = ref<Date | null>(null);
const reason = ref('');
const errorMessage = ref('');

const minDate = new Date(new Date().setDate(new Date().getDate() + 1));

function updateLicense() {
    const license = props.license;
    if (!license || !newExpiresAt.value) return;

    withLoading(async () => {
        try {
            errorMessage.value = '';
            await usersStore.updateUserLicense(props.userId, {
                type: license.type,
                publicId: license.publicId || undefined,
                bucketName: license.bucketName || undefined,
                expiresAt: license.expiresAt,
                newExpiresAt: (date.date(newExpiresAt.value) as Date).toISOString(),
                reason: reason.value,
            });
            notify.success('License updated successfully');
            model.value = false;
            emit('success');
        } catch (error) {
            errorMessage.value = error instanceof Error ? error.message : 'An unexpected error occurred';
            notify.error('Failed to update license', error);
        }
    });
}

watch(model, (newVal) => {
    if (newVal) {
        reason.value = '';
        newExpiresAt.value = null;
        errorMessage.value = '';
    }
});
</script>

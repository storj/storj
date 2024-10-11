// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        :persistent="isLoading"
        width="auto"
        max-width="420px"
        transition="fade-transition"
    >
        <v-card>
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <component :is="SquareAsterisk" :size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">Change Password</v-card-title>
                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </template>
            </v-card-item>
            <v-divider />
            <v-card-item class="px-6 pt-5">
                <p>You will receive a verification link in your email to confirm the password change.</p>
            </v-card-item>
            <v-card-item class="px-6 py-0">
                <v-form v-model="formValid" @submit.prevent="onChangePassword">
                    <v-col cols="12" class="px-0">
                        <v-text-field
                            v-model="oldPassword"
                            variant="outlined"
                            type="password"
                            :rules="oldRules"
                            label="Current password"
                            placeholder="Enter your current password"
                            class="mb-2"
                            :hide-details="false"
                            required
                            autofocus
                        />
                        <v-text-field
                            v-model="newPassword"
                            variant="outlined"
                            type="password"
                            :rules="newRules"
                            label="New password"
                            placeholder="Enter a new password"
                            class="mb-2"
                            :hide-details="false"
                            required
                        />
                        <v-text-field
                            variant="outlined"
                            type="password"
                            :rules="repeatRules"
                            label="Repeat password"
                            placeholder="Enter the new password again"
                            class="mb-2"
                            :hide-details="false"
                            required
                        />
                    </v-col>
                </v-form>
            </v-card-item>
            <v-divider />
            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            :disabled="isLoading"
                            @click="model = false"
                        >
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary"
                            variant="flat"
                            block
                            :disabled="!formValid"
                            :loading="isLoading"
                            @click="onChangePassword"
                        >
                            Save
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import {
    VDialog,
    VCard,
    VCardItem,
    VCardTitle,
    VDivider,
    VCardActions,
    VRow,
    VCol,
    VBtn,
    VForm,
    VTextField,
    VSheet,
} from 'vuetify/components';
import { SquareAsterisk } from 'lucide-vue-next';

import { RequiredRule } from '@/types/common';
import { useLoading } from '@/composables/useLoading';
import { useConfigStore } from '@/store/modules/configStore';
import { AuthHttpApi } from '@/api/auth';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';

const auth: AuthHttpApi = new AuthHttpApi();
const oldRules = [
    RequiredRule,
];
const newRules = [
    RequiredRule,
    (value: string) => (value && value.length >= config.passwordMinimumLength || `Invalid password. Use ${config.passwordMinimumLength} or more characters`),
    (value: string) => (value && value.length <= config.passwordMaximumLength || `Invalid password. Use ${config.passwordMaximumLength} or fewer characters`),
];
const repeatRules = [
    ...newRules,
    (value: string) => (value && value === newPassword.value || 'Passwords are not the same.'),
];

const analyticsStore = useAnalyticsStore();
const { config } = useConfigStore().state;
const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const model = defineModel<boolean>({ required: true });

const formValid = ref<boolean>(false);
const oldPassword = ref<string>('');
const newPassword = ref<string>('');

/**
 * Handles change password request.
 */
async function onChangePassword(): Promise<void> {
    if (!formValid.value) return;

    await withLoading(async () => {
        try {
            await auth.changePassword(oldPassword.value, newPassword.value);

            notify.success('Password successfully changed!');
            analyticsStore.eventTriggered(AnalyticsEvent.PASSWORD_CHANGED);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.CHANGE_PASSWORD_MODAL);
            return;
        }

        model.value = false;
    });
}
</script>

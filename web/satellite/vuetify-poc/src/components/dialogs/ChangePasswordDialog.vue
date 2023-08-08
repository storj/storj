// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        min-width="320px"
        max-width="410px"
        transition="fade-transition"
    >
        <v-card rounded="xlg">
            <v-card-item class="pl-7 pr-0 pb-5 pt-0">
                <v-row align="start" justify="space-between" class="ma-0">
                    <v-row align="center" class="ma-0 pt-5">
                        <img class="flex-shrink-0" src="@poc/assets/icon-change-password.svg" alt="Change password">
                        <v-card-title class="font-weight-bold ml-4">Change password</v-card-title>
                    </v-row>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </v-row>
            </v-card-item>
            <v-divider />
            <v-card-item class="px-7 py-5">
                <p>You will receive a verification link in your email to confirm the password change.</p>
            </v-card-item>
            <v-divider />
            <v-card-item class="px-7 py-5">
                <v-form v-model="formValid">
                    <v-col cols="12" class="px-0">
                        <v-text-field
                            v-model="oldPassword"
                            variant="outlined"
                            type="password"
                            :rules="oldRules"
                            label="Current password"
                            required
                            autofocus
                        />
                    </v-col>
                    <v-col cols="12" class="px-0">
                        <v-text-field
                            v-model="newPassword"
                            variant="outlined"
                            type="password"
                            :rules="newRules"
                            label="New password"
                            required
                        />
                    </v-col>
                    <v-col cols="12" class="px-0">
                        <v-text-field
                            variant="outlined"
                            type="password"
                            :rules="repeatRules"
                            label="Repeat password"
                            required
                        />
                    </v-col>
                </v-form>
            </v-card-item>
            <v-divider />
            <v-card-actions class="px-7 py-5">
                <v-row class="ma-0">
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            :disabled="isLoading"
                            :loading="isLoading"
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
                            :disabled="isLoading || !formValid"
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
import { computed, ref } from 'vue';
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
} from 'vuetify/components';
import { useRouter } from 'vue-router';

import { useLoading } from '@/composables/useLoading';
import { useConfigStore } from '@/store/modules/configStore';
import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/types/router';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';

const DELAY_BEFORE_REDIRECT = 2000; // 2 sec
const auth: AuthHttpApi = new AuthHttpApi();
const oldRules = [
    (value: string) => (value && value.length >= config.passwordMinimumLength || `Invalid old password. Must be ${config.passwordMinimumLength} or more characters`),
];
const newRules = [
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
const router = useRouter();
const notify = useNotify();

const props = defineProps<{
    modelValue: boolean,
}>();

const emit = defineEmits<{
    (event: 'update:modelValue', value: boolean): void,
}>();

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});
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

        try {
            await auth.logout();

            setTimeout(() => {
                router.push(RouteConfig.Login.path);
                // TODO: this reload will be unnecessary once vuetify poc has its own login and/or becomes the primary app
                location.reload();
            }, DELAY_BEFORE_REDIRECT);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.CHANGE_PASSWORD_MODAL);
        }

        emit('update:modelValue', false);
    });
}
</script>

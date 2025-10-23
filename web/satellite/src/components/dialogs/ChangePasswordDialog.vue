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
                        <component :is="Lock" :size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">Change Password</v-card-title>
                <template #append>
                    <v-btn
                        :icon="X"
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
                        <v-tooltip
                            v-model="showPasswordStrength"
                            width="500px"
                            location="bottom"
                            :open-on-hover="false"
                        >
                            <template #activator="{ props }">
                                <v-text-field
                                    v-model="newPassword"
                                    v-bind="props"
                                    variant="outlined"
                                    type="password"
                                    :rules="newRules"
                                    label="New password"
                                    placeholder="Enter a new password"
                                    class="mb-2"
                                    :hide-details="false"
                                    required
                                    @update:focused="showPasswordStrength = !showPasswordStrength"
                                />
                            </template>
                            <password-strength
                                :email="userEmail"
                                :password="newPassword"
                            />
                        </v-tooltip>
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
import { computed, onBeforeMount, ref, watch } from 'vue';
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
    VTooltip,
    VSheet,
} from 'vuetify/components';
import { Lock, X } from 'lucide-vue-next';

import { GoodPasswordRule, RequiredRule, ValidationRule } from '@/types/common';
import { useLoading } from '@/composables/useLoading';
import { useConfigStore } from '@/store/modules/configStore';
import { AuthHttpApi } from '@/api/auth';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { useUsersStore } from '@/store/modules/usersStore';

import PasswordStrength from '@/components/PasswordStrength.vue';

const auth: AuthHttpApi = new AuthHttpApi();

const configStore = useConfigStore();
const usersStore = useUsersStore();

const badPasswords = computed<Set<string>>(() => usersStore.state.badPasswords);
const liveCheckBadPassword = computed<boolean>(() => configStore.state.config.liveCheckBadPasswords);

const oldRules = [
    RequiredRule,
];

const newRules = computed<ValidationRule<string>[]>(() => {
    const rules = [
        RequiredRule,
        (value: string) => value.length < config.passwordMinimumLength || value.length > config.passwordMaximumLength
            ? `Password must be between ${config.passwordMinimumLength} and ${config.passwordMaximumLength} characters`
            : true,
    ];
    if (liveCheckBadPassword.value) rules.push(GoodPasswordRule);

    return rules;
});

const repeatRules = [
    ...newRules.value,
    (value: string) => (value && value === newPassword.value || 'Passwords are not the same.'),
];

const analyticsStore = useAnalyticsStore();
const { config } = useConfigStore().state;
const { isLoading, withLoading } = useLoading();
const notify = useNotify();
const userStore = useUsersStore();

const model = defineModel<boolean>({ required: true });

const formValid = ref<boolean>(false);
const showPasswordStrength = ref(false);
const oldPassword = ref<string>('');
const newPassword = ref<string>('');

const userEmail = computed<string>(() => userStore.state.user?.email ?? '');

/**
 * Handles change password request.
 */
async function onChangePassword(): Promise<void> {
    if (!formValid.value) return;

    await withLoading(async () => {
        try {
            await auth.changePassword(oldPassword.value, newPassword.value, config.csrfToken);

            notify.success('Password successfully changed!');
            analyticsStore.eventTriggered(AnalyticsEvent.PASSWORD_CHANGED);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.CHANGE_PASSWORD_MODAL);
            return;
        }

        model.value = false;
    });
}

onBeforeMount(() => {
    if (liveCheckBadPassword.value && badPasswords.value.size === 0) {
        usersStore.getBadPasswords().catch(() => {});
    }
});

watch(model, val => {
    if (!val) {
        oldPassword.value = '';
        newPassword.value = '';
        formValid.value = false;
    }
});
</script>

<style scoped lang="scss">
:deep(.v-overlay__content) {
    padding: 0 !important;
}
</style>

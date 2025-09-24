// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" width="600" transition="fade-transition">
        <v-card rounded="xlg">
            <template #title>
                Update Account
            </template>
            <template v-if="account.freezeStatus" #subtitle>
                This account is frozen, so updates to status and kind are disabled.
            </template>
            <template #append>
                <v-btn
                    icon="$close" :disabled="isLoading"
                    variant="text" size="small" color="default" @click="model = false"
                />
            </template>

            <v-form v-model="valid" @submit.prevent="update">
                <v-row class="px-6 pt-6">
                    <v-col v-if="featureFlags.updateEmail">
                        <v-text-field
                            v-model="email"
                            :error-messages="emailErrorMsg"
                            label="Account Email"
                            variant="solo-filled" flat
                            :rules="[RequiredRule, EmailRule]"
                            :disabled="isLoading"
                            hide-details="auto"
                            @update:model-value="onEmailChange"
                        />
                    </v-col>
                    <v-col v-if="featureFlags.updateName">
                        <v-text-field
                            v-model="name"
                            label="Account Name"
                            variant="solo-filled" flat
                            :rules="[RequiredRule]"
                            :disabled="isLoading"
                            hide-details="auto"
                        />
                    </v-col>
                </v-row>

                <v-row class="px-6">
                    <v-col v-if="featureFlags.updateKind && !account.freezeStatus">
                        <v-select
                            v-model="kind"
                            label="User kind"
                            placeholder="Select user kind"
                            :items="userKinds"
                            item-title="name"
                            item-value="value"
                            :rules="[RequiredRule]"
                            :disabled="isLoading"
                            variant="solo-filled"
                            flat required
                            hide-details="auto"
                        />
                    </v-col>
                    <v-col v-if="featureFlags.updateStatus && !account.freezeStatus">
                        <v-select
                            v-model="status"
                            label="User Status"
                            placeholder="Select user status"
                            :items="userStatuses"
                            item-title="name" item-value="value"
                            :rules="[RequiredRule]"
                            :disabled="isLoading"
                            variant="solo-filled"
                            flat required
                            hide-details="auto"
                        />
                    </v-col>
                </v-row>

                <v-row class="px-6 pb-6" justify="center">
                    <v-col v-if="featureFlags.updateKind && kind === UserKind.Free && !account.freezeStatus">
                        <v-date-input
                            v-model="trialExpiration"
                            label="Trial Expiration Date"
                            prepend-icon=""
                            variant="solo-filled"
                            hide-details="auto"
                            flat clearable
                            :min="useDate().addDays(new Date(), 1)"
                            :disabled="isLoading"
                        />
                    </v-col>
                    <v-col v-if="featureFlags.updateUserAgent">
                        <v-text-field
                            v-model="userAgent"
                            label="Useragent"
                            hide-details="auto"
                            variant="solo-filled"
                            :disabled="isLoading"
                            flat clearable
                        />
                    </v-col>
                </v-row>

                <v-card-actions class="pa-6">
                    <v-row>
                        <v-col>
                            <v-btn
                                variant="outlined" color="default"
                                :disabled="isLoading"
                                block @click="model = false"
                            >
                                Cancel
                            </v-btn>
                        </v-col>
                        <v-col>
                            <v-btn
                                color="primary"
                                variant="flat"
                                type="submit"
                                block
                                :disabled="!valid"
                                :loading="isLoading"
                                @click="update"
                            >
                                Update
                            </v-btn>
                        </v-col>
                    </v-row>
                </v-card-actions>
            </v-form>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import {
    VBtn,
    VCard,
    VCardActions,
    VCol,
    VDialog,
    VForm,
    VRow,
    VSelect,
    VTextField,
} from 'vuetify/components';
import { VDateInput } from 'vuetify/labs/VDateInput';
import { useDate } from 'vuetify';
import { computed, ref, watch } from 'vue';

import { UpdateUserRequest, UserAccount } from '@/api/client.gen';
import { useLoading } from '@/composables/useLoading';
import { useUsersStore } from '@/store/users';
import { EmailRule, RequiredRule } from '@/types/common';
import { useNotify } from '@/composables/useNotify';
import { useAppStore } from '@/store/app';
import { UserKind } from '@/types/user';

const appStore = useAppStore();
const usersStore = useUsersStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    account: UserAccount;
}>();

const name = ref(props.account.fullName);
const email = ref(props.account.email);
const kind = ref(props.account.kind.value);
const trialExpiration = ref(props.account.trialExpiration ? new Date(props.account.trialExpiration) : null);
const status = ref(props.account.status.value);
const userAgent = ref(props.account.userAgent);
const valid = ref(false);

const emailErrorMsg = ref<string>();
let emailCheckTimer: ReturnType<typeof setTimeout> | undefined;

const userStatuses = computed(() => usersStore.state.userStatuses);
const userKinds = computed(() => usersStore.state.userKinds);
const featureFlags = computed(() => appStore.state.settings.admin.features.account);

function update() {
    if (!valid.value || isLoading.value)
        return;

    const request = new UpdateUserRequest();
    if (featureFlags.value.updateUserAgent) request.userAgent = userAgent.value ?? '';
    if (featureFlags.value.updateName) request.name = name.value;
    if (featureFlags.value.updateKind && !props.account.freezeStatus) {
        request.kind = kind.value;
        if (kind.value === UserKind.Free) request.trialExpiration = trialExpiration.value?.toISOString() ?? null;
    }
    if (featureFlags.value.updateStatus && !props.account.freezeStatus) request.status = status.value;
    if (featureFlags.value.updateEmail) request.email = email.value;

    withLoading(async () => {
        try {
            const account = await usersStore.updateUser(props.account.id, request);
            await usersStore.updateCurrentUser(account);
            model.value = false;
            notify.success('Account updated successfully!');
        } catch (e) {
            notify.error(`Failed to update account. ${e.message}`);
        }
    });
}

function onEmailChange(newEmail: string) {
    clearTimeout(emailCheckTimer);
    emailErrorMsg.value = undefined;
    if (EmailRule(newEmail) !== true || newEmail === props.account.email) {
        return;
    }
    emailCheckTimer = setTimeout(() => checkEmailAvailability(newEmail), 700);
}

function checkEmailAvailability(newEmail: string) {
    withLoading(async () => {
        try {
            await usersStore.getUserByEmail(newEmail);
            emailErrorMsg.value = 'This email is already in use by another account.';
        } catch (e) {
            if (e.responseStatusCode !== 404) {
                emailErrorMsg.value = 'Error checking email availability.';
            }
            // 404 means email is available, so do nothing
        }
    });
}

watch(model, (newValue) => {
    usersStore.getUserKinds();
    usersStore.getUserStatuses();

    if (!newValue) return;
    name.value = props.account.fullName;
    email.value = props.account.email;
    kind.value = props.account.kind.value;
    trialExpiration.value = props.account.trialExpiration ? new Date(props.account.trialExpiration) : null;
    status.value = props.account.status.value;
    userAgent.value = props.account.userAgent;
    emailErrorMsg.value = undefined;
});
</script>
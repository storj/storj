// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <RequireReasonFormDialog
        v-model="model"
        :loading="isLoading"
        :initial-form-data="initialFormData"
        :form-config="formConfig"
        title="Update Account"
        :subtitle="account.freezeStatus ? 'This account is frozen, so updates to status and kind are disabled.' : ''"
        width="600"
        @submit="update"
    />
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { useDate } from 'vuetify/framework';

import { UpdateUserRequest, UserAccount } from '@/api/client.gen';
import { useLoading } from '@/composables/useLoading';
import { useUsersStore } from '@/store/users';
import { EmailRule, RequiredRule } from '@/types/common';
import { useNotify } from '@/composables/useNotify';
import { useAppStore } from '@/store/app';
import { UserKind, UserStatus } from '@/types/user';
import { FieldType, FormConfig, FormField } from '@/types/forms';

import RequireReasonFormDialog from '@/components/RequireReasonFormDialog.vue';

const appStore = useAppStore();
const usersStore = useUsersStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();
const date = useDate();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    account: UserAccount;
}>();

const emailErrorMsg = ref<string>();
let emailCheckTimer: ReturnType<typeof setTimeout> | undefined;

const userStatuses = computed(() => usersStore.state.userStatuses);
const userKinds = computed(() => usersStore.state.userKinds);
const featureFlags = computed(() => appStore.state.settings.admin.features.account);

const initialFormData = computed(() => ({
    email: props.account?.email ?? '',
    name: props.account?.fullName ?? '',
    kind: props.account?.kind?.value ?? UserKind.Free.valueOf(),
    status: props.account?.status?.value ?? 0,
    trialExpiration: props.account?.trialExpiration,
    userAgent: props.account?.userAgent ?? '',
}));

const formConfig = computed((): FormConfig => {
    const config: FormConfig = {
        sections: [{ rows: [] }],
    };

    const firstRowFields: FormField[] = [];
    if (featureFlags.value.updateEmail) firstRowFields.push({
        key: 'email',
        type: FieldType.Text,
        label: 'Account Email',
        rules: [RequiredRule, EmailRule],
        errorMessages: () => emailErrorMsg.value,
        onUpdate: (value) => onEmailChange(value as string),
    });
    if (featureFlags.value.updateName) firstRowFields.push({
        key: 'name',
        type: FieldType.Text,
        label: 'Account Name',
        rules: [RequiredRule],
    });
    if (firstRowFields.length > 0) config.sections[0].rows.push({ fields: firstRowFields });

    const secondRowFields: FormField[] = [];
    if (featureFlags.value.updateKind && !props.account?.freezeStatus)
        secondRowFields.push({
            key: 'kind',
            type: FieldType.Select,
            label: 'User kind',
            placeholder: 'Select user kind',
            items: userKinds.value,
            itemTitle: 'name',
            itemValue: 'value',
            rules: [RequiredRule],
            required: true,
        });
    if (featureFlags.value.updateStatus && !props.account?.freezeStatus)
        secondRowFields.push({
            key: 'status',
            type: FieldType.Select,
            label: 'User Status',
            placeholder: 'Select user status',
            items: props.account.status.value === UserStatus.PendingDeletion ?
                userStatuses.value :
                userStatuses.value.filter(s => s.value !== UserStatus.PendingDeletion),
            itemTitle: 'name',
            itemValue: 'value',
            rules: [RequiredRule],
            required: true,
        });
    if (secondRowFields.length > 0) config.sections[0].rows.push({ fields: secondRowFields });

    const thirdRowFields: FormField[] = [];
    if (featureFlags.value.updateKind && !props.account?.freezeStatus)
        thirdRowFields.push({
            key: 'trialExpiration',
            type: FieldType.Date,
            label: 'Trial Expiration Date',
            clearable: true,
            prependIcon: '',
            min: date.addDays(new Date(), 1) as Date,
            visible: (formData) => (formData as { kind: UserKind }).kind === UserKind.Free,
            transform: {
                forward: (value) => value ? date.date(value): null,
                back: (value) => {
                    return value ? (date.date(value) as Date).toISOString() : '';
                },
            },
        });
    if (featureFlags.value.updateUserAgent) thirdRowFields.push({
        key: 'userAgent',
        type: FieldType.Text,
        label: 'Useragent',
        clearable: true,
        transform: {
            back: (value) => value ?? '',
        },
    });
    if (thirdRowFields.length > 0) config.sections[0].rows.push({ fields: thirdRowFields });

    return config;
});

function update(formData: Record<string, unknown>) {
    withLoading(async () => {
        const request = new UpdateUserRequest();
        for (const key in request) {
            if (!Object.hasOwn(formData, key)) continue;
            if (formData[key] === initialFormData.value[key]) continue;
            // set only changed fields
            request[key] = formData[key];
        }

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
    if (!newValue) return;
    emailErrorMsg.value = undefined;
});

onMounted(() => {
    withLoading(async () => {
        try {
            await Promise.all([
                usersStore.getUserKinds(),
                usersStore.getUserStatuses(),
            ]);
        } catch (e) {
            notify.error(e);
        }
    });
});
</script>

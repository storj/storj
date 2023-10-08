// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        max-width="420px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card rounded="xlg">
            <v-sheet>
                <v-card-item class="pl-7 py-4">
                    <template #prepend>
                        <v-card-title class="font-weight-bold">
                            Add Members
                        </v-card-title>
                    </template>

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
            </v-sheet>

            <v-divider />

            <v-form v-model="valid" class="pa-7 pb-4" @submit.prevent="onAddUsersClick">
                <v-row>
                    <v-col cols="12">
                        <p class="mb-5">Invite team members to join you in this project.</p>
                        <v-alert
                            variant="tonal"
                            color="info"
                            title="Important Information"
                            text="All team members should use the same passphrase to access the same data."
                            rounded="lg"
                            density="comfortable"
                            border
                        />
                    </v-col>
                    <v-col cols="12">
                        <v-text-field
                            v-model="email"
                            variant="outlined"
                            :rules="emailRules"
                            label="Enter e-mail"
                            hint="Members will have read & write permissions."
                            required
                            autofocus
                            class="my-2"
                        />
                    </v-col>
                </v-row>
            </v-form>

            <v-divider />

            <v-card-actions class="pa-7">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block :disabled="isLoading" @click="model = false">Cancel</v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="primary" variant="flat" block :loading="isLoading" @click="onAddUsersClick">Send Invite</v-btn>
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
    VSheet,
    VCardItem,
    VCardTitle,
    VBtn,
    VDivider,
    VForm,
    VRow,
    VCol,
    VAlert,
    VTextField,
    VCardActions,
} from 'vuetify/components';

import { RequiredRule, ValidationRule } from '@poc/types/common';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useNotify } from '@/utils/hooks';
import { useLoading } from '@/composables/useLoading';

const props = defineProps<{
    modelValue: boolean,
    projectId: string,
}>();

const emit = defineEmits<{
    'update:modelValue': [value: boolean],
}>();

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

const analyticsStore = useAnalyticsStore();
const pmStore = useProjectMembersStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const valid = ref<boolean>(false);
const email = ref<string>('');

const emailRules: ValidationRule<string>[] = [
    RequiredRule,
    v => ((/.+@.+\..+/.test(v)) || 'E-mail must be valid.'),
];

/**
 * Sends a project invitation to the input email.
 */
async function onAddUsersClick(): Promise<void> {
    if (!valid.value) return;

    await withLoading(async () => {
        try {
            await pmStore.inviteMembers([email.value], props.projectId);
            notify.notify('Invites sent!');
            email.value = '';
        } catch (error) {
            error.message = `Error adding project members. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.ADD_PROJECT_MEMBER_MODAL);
            return;
        }

        analyticsStore.eventTriggered(AnalyticsEvent.PROJECT_MEMBERS_INVITE_SENT);

        try {
            await pmStore.getProjectMembers(1, props.projectId);
        } catch (error) {
            error.message = `Unable to fetch project members. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.ADD_PROJECT_MEMBER_MODAL);
        }

        model.value = false;
    });
}
</script>

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        :persistent="isLoading"
        width="auto"
        min-width="320px"
        max-width="410px"
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
                        <component :is="UserPen" :size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">Edit Name</v-card-title>
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
            <v-card-item>
                <v-form v-model="formValid" @submit.prevent="onChangeName">
                    <v-col cols="12" class="px-0">
                        <v-text-field
                            v-model="name"
                            variant="outlined"
                            :rules="rules"
                            label="Full name"
                            placeholder="Enter your name"
                            :hide-details="false"
                            maxlength="72"
                            required
                            autofocus
                            class="mt-2"
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
                            @click="onChangeName"
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
import { UserPen, X } from 'lucide-vue-next';

import { useLoading } from '@/composables/useLoading';
import { useUsersStore } from '@/store/modules/usersStore';
import { UpdatedUser } from '@/types/users';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useNotify } from '@/composables/useNotify';

const rules = [
    (value: string) => (!!value || 'Can\'t be empty'),
];

const analyticsStore = useAnalyticsStore();
const userStore = useUsersStore();
const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const model = defineModel<boolean>({ required: true });

const formValid = ref<boolean>(false);
const name = ref<string>(userStore.userName);

/**
 * Handles change name request.
 */
async function onChangeName(): Promise<void> {
    if (!formValid.value) return;

    await withLoading(async () => {
        try {
            await userStore.updateUser(new UpdatedUser(name.value, name.value));

            notify.success('Account info successfully updated!');
            analyticsStore.eventTriggered(AnalyticsEvent.PROFILE_UPDATED);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.EDIT_PROFILE_MODAL);
            return;
        }

        model.value = false;
    });
}
</script>

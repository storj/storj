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
            <v-card-item class="pl-7 py-4">
                <template #prepend>
                    <img class="d-block" src="@poc/assets/icon-change-name.svg" alt="Change name">
                </template>
                <v-card-title class="font-weight-bold">Edit Name</v-card-title>
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
            <v-card-item class="px-7 py-5">
                <v-form v-model="formValid" @submit.prevent="onChangeName">
                    <v-col cols="12" class="px-0">
                        <v-text-field
                            v-model="name"
                            variant="outlined"
                            :rules="rules"
                            label="Full name"
                            :hide-details="false"
                            required
                            autofocus
                        />
                    </v-col>
                </v-form>
            </v-card-item>
            <v-divider />
            <v-card-actions class="px-7 py-5">
                <v-row class="ma-0">
                    <v-col class="pl-0">
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
                    <v-col class="pr-0">
                        <v-btn
                            color="primary"
                            variant="flat"
                            block
                            :disabled="isLoading || !formValid"
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

import { useLoading } from '@/composables/useLoading';
import { useUsersStore } from '@/store/modules/usersStore';
import { UpdatedUser } from '@/types/users';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useNotify } from '@/utils/hooks';

const rules = [
    (value: string) => (!!value || 'Can\'t be empty'),
];

const analyticsStore = useAnalyticsStore();
const userStore = useUsersStore();
const { isLoading, withLoading } = useLoading();
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

        emit('update:modelValue', false);
    });
}
</script>

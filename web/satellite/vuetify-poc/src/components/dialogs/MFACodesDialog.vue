// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        min-width="320px"
        max-width="460px"
        transition="fade-transition"
    >
        <v-card rounded="xlg">
            <v-card-item class="pl-7 py-4">
                <template #prepend>
                    <img class="d-block" src="@poc/assets/icon-mfa.svg" alt="MFA">
                </template>
                <v-card-title class="font-weight-bold">Two-Factor Recovery Codes</v-card-title>
                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        @click="model = false"
                    />
                </template>
            </v-card-item>
            <v-divider class="mx-8" />
            <v-card-item class="px-8 py-4">
                <p>Please save these codes somewhere to be able to recover access to your account.</p>
            </v-card-item>
            <v-divider class="mx-8" />
            <v-card-item class="px-8 py-4">
                <p
                    v-for="(code, index) in userMFARecoveryCodes"
                    :key="index"
                >
                    {{ code }}
                </p>
            </v-card-item>
            <v-divider class="mx-8 mb-4" />
            <v-card-actions dense class="px-7 pb-5 pt-0">
                <v-col class="px-0">
                    <v-btn
                        color="primary"
                        variant="flat"
                        block
                        @click="model = false"
                    >
                        Done
                    </v-btn>
                </v-col>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VCol,
    VDialog,
    VDivider,
    VRow,
} from 'vuetify/components';

import { AuthHttpApi } from '@/api/auth';
import { useUsersStore } from '@/store/modules/usersStore';

const auth: AuthHttpApi = new AuthHttpApi();

const usersStore = useUsersStore();

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

/**
 * Returns user MFA recovery codes from store.
 */
const userMFARecoveryCodes = computed((): string[] => {
    return usersStore.state.userMFARecoveryCodes;
});
</script>
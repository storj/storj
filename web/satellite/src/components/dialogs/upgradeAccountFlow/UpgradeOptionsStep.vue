// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <p class="pt-2 pb-4">
        Add a credit card to activate your Pro Account, or deposit more than $10 in STORJ tokens to upgrade
        and get 10% bonus on your STORJ tokens deposit.
    </p>

    <v-alert v-if="user.trialExpiration" border class="my-2" type="info" variant="tonal" color="info">
        <p class="text-body-2 my-2">By upgrading, your trial will end immediately.</p>
    </v-alert>

    <v-row justify="center" class="pb-5 pt-3">
        <v-col>
            <v-btn
                variant="flat"
                color="primary"
                block
                :loading="loading"
                @click="emit('addCard')"
            >
                <template #prepend>
                    <v-icon :icon="mdiCreditCard" />
                </template>
                Add Credit Card
            </v-btn>
        </v-col>
        <v-col>
            <v-btn
                variant="flat"
                block
                :loading="loading"
                @click="emit('addTokens')"
            >
                <template #prepend>
                    <v-icon :icon="mdiPlusCircle" />
                </template>
                Add STORJ Tokens
            </v-btn>
        </v-col>
    </v-row>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VBtn, VCol, VIcon, VRow, VAlert } from 'vuetify/components';
import { mdiCreditCard, mdiPlusCircle } from '@mdi/js';

import { useUsersStore } from '@/store/modules/usersStore';
import { User } from '@/types/users';

const usersStore = useUsersStore();

defineProps<{
    loading: boolean;
}>();

const emit = defineEmits<{
    addCard: [];
    addTokens: [];
}>();

/**
 * Returns user entity from store.
 */
const user = computed<User>(() => usersStore.state.user);
</script>

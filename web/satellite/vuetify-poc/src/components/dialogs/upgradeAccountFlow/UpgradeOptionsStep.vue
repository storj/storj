// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <p class="pt-2 pb-4">
        Add a credit card to activate your Pro Account, or deposit more than $10 in STORJ tokens to upgrade
        and get 10% bonus on your STORJ tokens deposit.
    </p>

    <v-row justify="center" class="mx-0 pb-5 pt-4">
        <v-col class="pl-0">
            <v-btn
                variant="flat"
                color="primary"
                block
                :loading="loading"
                @click="emit('addCard')"
            >
                <template #prepend>
                    <v-icon icon="mdi-credit-card" />
                </template>
                Add Credit Card
            </v-btn>
        </v-col>
        <v-col class="px-0">
            <v-btn
                variant="flat"
                block
                :loading="loading"
                @click="emit('addTokens')"
            >
                <template #prepend>
                    <v-icon icon="mdi-plus-circle" />
                </template>
                Add STORJ Tokens
            </v-btn>
        </v-col>
    </v-row>
</template>

<script setup lang="ts">
import { VBtn, VCol, VDivider, VIcon, VRow } from 'vuetify/components';

import { useUsersStore } from '@/store/modules/usersStore';
import { useNotify } from '@/utils/hooks';

const usersStore = useUsersStore();
const notify = useNotify();

const props = defineProps<{
    loading: boolean;
}>();

const emit = defineEmits<{
    addCard: [];
    addTokens: [];
}>();
</script>
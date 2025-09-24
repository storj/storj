// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" width="400" transition="fade-transition">
        <v-card rounded="xlg">
            <template #title>
                Freeze Account
            </template>
            <template #subtitle>
                Select the freeze type to apply.
            </template>
            <template #append>
                <v-btn :icon="X" variant="text" size="small" color="default" @click="model = false" />
            </template>

            <v-form class="pa-6">
                <v-row>
                    <v-col cols="12">
                        <v-select
                            v-model="freezeType"
                            label="Freeze type" placeholder="Select freeze type"
                            :items="freezeTypes"
                            :disabled="isLoading"
                            item-title="name" item-value="value"
                            hide-details="auto"
                            variant="solo-filled"
                            flat required
                        />
                    </v-col>
                </v-row>
            </v-form>

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined" color="default"
                            :disabled="isLoading"
                            block
                            @click="model = false"
                        >
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="warning" variant="flat"
                            :loading="isLoading"
                            :disabled="freezeType === undefined"
                            block
                            @click="freezeAccount"
                        >
                            Freeze Account
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
    VDialog,
    VCard,
    VBtn,
    VForm,
    VRow,
    VCol,
    VSelect,
    VCardActions,
} from 'vuetify/components';
import { X } from 'lucide-vue-next';

import { UserAccount } from '@/api/client.gen';
import { useUsersStore } from '@/store/users';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';

const usersStore = useUsersStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const freezeType = ref<number>();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    account: UserAccount;
}>();

const freezeTypes = computed(() => usersStore.state.freezeTypes);

function freezeAccount() {
    withLoading(async () => {
        if (freezeType.value === undefined) {
            return;
        }
        try {
            await usersStore.freezeUser(props.account.id, freezeType.value);
            await usersStore.updateCurrentUser(props.account.id);
            notify.success('Account frozen successfully.');
            model.value = false;
        } catch (error) {
            notify.error(`Failed to freeze account. ${error.message}`);
            return;
        }
    });
}

watch(() => model.value, (newVal) => {
    if (!newVal) {
        freezeType.value = undefined;
    }
});
</script>
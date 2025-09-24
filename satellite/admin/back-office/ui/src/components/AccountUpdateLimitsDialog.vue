// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" width="550" transition="fade-transition">
        <v-card rounded="xlg">
            <template #title>
                Update Account Default Limits
            </template>
            <template #subtitle>
                Enter default limits per project for this account.
            </template>
            <template #append>
                <v-btn
                    :icon="X" :disabled="isLoading"
                    variant="text" size="small" color="default" @click="model = false"
                />
            </template>

            <v-form v-model="valid" @submit.prevent="update">
                <v-row class="px-6 pt-6">
                    <v-col>
                        <v-number-input
                            v-model="projectLimit"
                            label="Total projects"
                            :rules="[RequiredRule, PositiveNumberRule]"
                            :disabled="isLoading"
                            hide-details="auto"
                            control-variant="stacked"
                            variant="solo-filled" flat
                        />
                    </v-col>
                    <v-col>
                        <v-number-input
                            v-model="segmentLimit"
                            label="Segments / project"
                            :rules="[RequiredRule, PositiveNumberRule]"
                            :disabled="isLoading"
                            hide-details="auto"
                            control-variant="stacked"
                            variant="solo-filled" flat
                            :step="5000"
                        />
                    </v-col>
                </v-row>
                <v-row class="px-6">
                    <v-col>
                        <v-number-input
                            v-model="storageLimitTB"
                            label="Storage / project"
                            suffix="TB"
                            :messages="[`Bytes: ${storageLimit}`]"
                            :precision="4" :step="0.5"
                            :rules="[RequiredRule, PositiveNumberRule, BytesMustBeWholeRule]"
                            :disabled="isLoading"
                            hide-details="auto"
                            control-variant="stacked"
                            variant="solo-filled" flat
                        />
                    </v-col>
                    <v-col>
                        <v-number-input
                            v-model="bandwidthLimitTB"
                            label="Download / month / project"
                            suffix="TB"
                            :messages="[`Bytes: ${bandwidthLimit}`]"
                            :precision="4" :step="0.5"
                            :rules="[RequiredRule, PositiveNumberRule, BytesMustBeWholeRule]"
                            :disabled="isLoading"
                            hide-details="auto"
                            control-variant="stacked"
                            variant="solo-filled" flat
                        />
                    </v-col>
                </v-row>
                <v-row class="px-6 pb-6">
                    <v-col cols="12">
                        <v-text-field
                            :model-value="account.email" label="Account Email"
                            variant="solo-filled" flat readonly
                            :disabled="isLoading"
                            hide-details="auto"
                        />
                    </v-col>
                </v-row>

                <v-card-actions class="pa-6">
                    <v-row>
                        <v-col>
                            <v-btn
                                variant="outlined"
                                color="default" block
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
                                type="submit"
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
import { computed, ref, watch } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCol,
    VDialog,
    VForm,
    VNumberInput,
    VRow,
    VTextField,
} from 'vuetify/components';
import { X } from 'lucide-vue-next';

import { useUsersStore } from '@/store/users';
import { useNotify } from '@/composables/useNotify';
import { useLoading } from '@/composables/useLoading';
import { UpdateUserRequest, UserAccount } from '@/api/client.gen';
import { BytesMustBeWholeRule, PositiveNumberRule, RequiredRule } from '@/types/common';
import { Memory } from '@/utils/bytesSize';

const usersStore = useUsersStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    account: UserAccount;
}>();

const valid = ref(false);
const projectLimit = ref(props.account.projectLimit);
const storageLimit = ref(props.account.storageLimit);
const bandwidthLimit = ref(props.account.bandwidthLimit);
const segmentLimit = ref(props.account.segmentLimit);

const storageLimitTB = computed({
    get: () => storageLimit.value / Memory.TB,
    set: (val: number) => storageLimit.value = val * Memory.TB,
});

const bandwidthLimitTB = computed({
    get: () => bandwidthLimit.value / Memory.TB,
    set: (val: number) => bandwidthLimit.value = val * Memory.TB,
});

function update() {
    if (!valid.value) {
        return;
    }

    withLoading(async () => {
        const request = new UpdateUserRequest();
        request.projectLimit = projectLimit.value;
        request.storageLimit = storageLimit.value;
        request.bandwidthLimit = bandwidthLimit.value;
        request.segmentLimit = segmentLimit.value;

        try {
            const account = await usersStore.updateUser(props.account.id,request);
            await usersStore.updateCurrentUser(account);

            model.value = false;
            notify.success('Limits updated successfully!');
        } catch (e) {
            notify.error(`Failed to update limits. ${e.message}`);
        }
    });
}

watch(model, (_) => {
    projectLimit.value = props.account.projectLimit;
    storageLimit.value = props.account.storageLimit;
    bandwidthLimit.value = props.account.bandwidthLimit;
    segmentLimit.value = props.account.segmentLimit;
});
</script>
// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pa-6">
        <v-row>
            <v-col cols="12">
                The encryption passphrase will be used to encrypt the objects you upload in this project.
                You can generate a new encryption passphrase, or enter your own.
            </v-col>
            <v-col cols="12">
                <p class="text-subtitle-2 font-weight-bold mb-2">Encryption Passphrase</p>
                <v-radio-group v-model="passphraseOption" hide-details="auto">
                    <v-radio label="Enter a new passphrase" :value="PassphraseOption.EnterPassphrase" />
                    <v-radio label="Generate a 12-word passphrase" :value="PassphraseOption.GeneratePassphrase" />
                </v-radio-group>
            </v-col>
        </v-row>
    </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import { VRow, VCol, VRadioGroup, VRadio } from 'vuetify/components';

import { DialogStepComponent } from '@/types/common';
import { PassphraseOption } from '@/types/managePassphrase';

const emit = defineEmits<{
    'selectOption': [option: PassphraseOption];
}>();

const passphraseOption = ref<PassphraseOption | null>(null);

watch(passphraseOption, value => value !== null && emit('selectOption', value));

defineExpose<DialogStepComponent>({
    title: 'Encryption Passphrase',
    onEnter: () => {
        if (passphraseOption.value !== null) return;
        passphraseOption.value = PassphraseOption.EnterPassphrase;
    },
});
</script>

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form ref="form" class="pa-8">
        <v-row>
            <v-col cols="12">
                <p v-if="passphraseType === CreateAccessStep.EnterMyPassphrase">
                    Enter the encryption passphrase used for this project to create this access grant.
                </p>
                <p v-else>
                    This passphrase will be used to encrypt all the files you upload using this access grant.
                    You will need it to access these files in the future.
                </p>
            </v-col>

            <v-col cols="12">
                <v-text-field
                    v-model="passphrase"
                    label="Encryption Passphrase"
                    :append-inner-icon="isPassphraseVisible ? 'mdi-eye-off' : 'mdi-eye'"
                    :type="isPassphraseVisible ? 'text' : 'password'"
                    variant="outlined"
                    hide-details="auto"
                    :rules="[ RequiredRule ]"
                    @click:append-inner="isPassphraseVisible = !isPassphraseVisible"
                />
            </v-col>

            <v-col v-if="passphraseType !== CreateAccessStep.EnterMyPassphrase" cols="12">
                <v-checkbox
                    density="compact"
                    color="primary"
                    label="Yes, I saved my encryption passphrase."
                    hide-details="auto"
                    :rules="[ RequiredRule ]"
                />
            </v-col>
        </v-row>
    </v-form>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import { VForm, VRow, VCol, VTextField, VCheckbox } from 'vuetify/components';

import { CreateAccessStep } from '@/types/createAccessGrant';
import { CreateAccessStepComponent } from '@poc/types/createAccessGrant';
import { RequiredRule } from '@poc/types/common';

const props = defineProps<{
    passphraseType: CreateAccessStep.EnterMyPassphrase | CreateAccessStep.EnterNewPassphrase,
}>();

const form = ref<VForm | null>(null);

const passphrase = ref<string>('');
const isPassphraseVisible = ref<boolean>(false);

const emit = defineEmits<{
    'passphraseChanged': [passphrase: string];
}>();

watch(passphrase, value => emit('passphraseChanged', value));

defineExpose<CreateAccessStepComponent>({
    title: props.passphraseType === CreateAccessStep.EnterMyPassphrase ? 'Enter Passphrase' : 'Enter New Passphrase',
    validate: () => {
        form.value?.validate();
        return !!form.value?.isValid;
    },
});
</script>

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form ref="form" class="pa-8">
        <v-row>
            <v-col cols="12">
                <p class="text-subtitle-2 font-weight-bold mb-2">Encryption Passphrase</p>
                <v-radio-group v-model="passphraseOption" :rules="[ RequiredRule ]" hide-details="auto">
                    <v-radio v-if="isPromptForPassphrase" label="Enter your project passphrase" :value="PassphraseOption.SetMyProjectPassphrase">
                        <template #label>
                            Enter your project passphrase
                            <info-tooltip>
                                You will enter your encryption passphrase on the next step.
                                Make sure it's the same one you use for this project.
                                This will allow you to manage existing data you have uploaded with the same passphrase.
                            </info-tooltip>
                        </template>
                    </v-radio>
                    <v-radio v-else :value="PassphraseOption.UseExistingPassphrase">
                        <template #label>
                            Use the current passphrase
                            <info-tooltip>
                                Create this access with the same passphrase you use for this project.
                                This allows you to manage existing data you have uploaded with the same passphrase.
                            </info-tooltip>
                        </template>
                    </v-radio>
                    <v-btn
                        class="align-self-start"
                        variant="text"
                        color="default"
                        :append-icon="areAdvancedOptionsShown ? '$collapse' : '$expand'"
                        :disabled="isAdvancedOptionSelected"
                        @click="areAdvancedOptionsShown = !areAdvancedOptionsShown"
                    >
                        Advanced
                    </v-btn>
                    <v-expand-transition>
                        <div v-show="areAdvancedOptionsShown">
                            <v-radio :value="PassphraseOption.EnterNewPassphrase">
                                <template #label>
                                    Enter a new passphrase
                                    <info-tooltip>
                                        Create this access with a new encryption passphrase that you can enter on the next step.
                                        The access will not be able to manage any existing data.
                                    </info-tooltip>
                                </template>
                            </v-radio>
                            <v-radio label="" :value="PassphraseOption.GenerateNewPassphrase">
                                <template #label>
                                    Generate a 12-word passphrase
                                    <info-tooltip>
                                        Create this access with a new encryption passphrase that will be generated for you on the next step.
                                        The access will not be able to manage any existing data.
                                    </info-tooltip>
                                </template>
                            </v-radio>
                        </div>
                    </v-expand-transition>
                </v-radio-group>
            </v-col>
            <v-expand-transition>
                <v-col v-show="areAdvancedOptionsShown" cols="12">
                    <v-alert type="warning" variant="tonal" rounded="xlg">
                        Creating a new passphrase for this access will prevent it from accessing any data
                        that has been uploaded with the current passphrase.
                    </v-alert>
                </v-col>
            </v-expand-transition>
        </v-row>
    </v-form>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
    VForm,
    VRow,
    VCol,
    VRadioGroup,
    VRadio,
    VBtn,
    VExpandTransition,
    VAlert,
} from 'vuetify/components';

import { PassphraseOption } from '@/types/createAccessGrant';
import { CreateAccessStepComponent } from '@poc/types/createAccessGrant';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { RequiredRule } from '@poc/types/common';

import InfoTooltip from '@poc/components/dialogs/createAccessSteps/InfoTooltip.vue';

const emit = defineEmits<{
    'selectOption': [option: PassphraseOption];
    'passphraseChanged': [passphrase: string];
}>();

const form = ref<VForm | null>(null);
const passphraseOption = ref<PassphraseOption>();

watch(passphraseOption, value => value && emit('selectOption', value));

const bucketsStore = useBucketsStore();

/**
 * Indicates whether the user should be prompted to enter the project passphrase.
 */
const isPromptForPassphrase = computed<boolean>(() => bucketsStore.state.promptForPassphrase);

const areAdvancedOptionsShown = ref<boolean>(isPromptForPassphrase.value);

/**
 * Indicates whether an option in the Advanced menu has been selected.
 */
const isAdvancedOptionSelected = computed<boolean>(() => {
    return passphraseOption.value === PassphraseOption.EnterNewPassphrase
        || passphraseOption.value === PassphraseOption.GenerateNewPassphrase;
});

defineExpose<CreateAccessStepComponent>({
    title: 'Access Encryption',
    validate: () => {
        form.value?.validate();
        return !!form.value?.isValid;
    },
    onExit: () => {
        if (passphraseOption.value !== PassphraseOption.UseExistingPassphrase) return;
        emit('passphraseChanged', bucketsStore.state.passphrase);
    },
});
</script>

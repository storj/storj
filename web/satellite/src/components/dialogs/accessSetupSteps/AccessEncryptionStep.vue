// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form ref="form" class="pa-6 pb-4" @submit.prevent="emit('submit')">
        <v-row>
            <v-col cols="12">
                <v-radio-group v-model="passphraseOption" hide-details="auto">
                    <template v-if="isPromptForPassphrase">
                        <v-radio v-if="isPromptForPassphrase" label="Enter your project passphrase" :value="PassphraseOption.SetMyProjectPassphrase" density="compact">
                            <template #label>
                                Enter my project passphrase
                                <info-tooltip>
                                    Make sure it's the same passphrase you use for this project.
                                    This will allow you to manage existing data you have uploaded.
                                </info-tooltip>
                            </template>
                        </v-radio>
                        <v-text-field
                            v-model="passphrase"
                            class="mt-6"
                            variant="outlined"
                            label="Encryption Passphrase"
                            :type="isPassphraseVisible ? 'text' : 'password'"
                            :rules="passphraseRules"
                            :hide-details="false"
                            autofocus
                            required
                        >
                            <template #append-inner>
                                <password-input-eye-icons
                                    :is-visible="isPassphraseVisible"
                                    type="passphrase"
                                    @toggle-visibility="isPassphraseVisible = !isPassphraseVisible"
                                />
                            </template>
                        </v-text-field>
                    </template>
                    <v-radio v-else :value="PassphraseOption.UseExistingPassphrase" density="compact" class="pb-4">
                        <template #label>
                            Use the current passphrase
                            <info-tooltip>
                                Create this access with the same passphrase you use for this project.
                                This allows you to manage existing data you have uploaded with the same passphrase.
                            </info-tooltip>
                        </template>
                    </v-radio>
                    <v-btn
                        class="align-self-start mt-2 mb-4"
                        variant="outlined"
                        color="default"
                        :append-icon="areAdvancedOptionsShown ? '$collapse' : '$expand'"
                        :disabled="isAdvancedOptionSelected"
                        @click="areAdvancedOptionsShown = !areAdvancedOptionsShown"
                    >
                        Advanced Options
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
                    <v-alert class="mb-4" type="info" variant="tonal">
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
    VTextField,
} from 'vuetify/components';

import { PassphraseOption } from '@/types/setupAccess';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { ValidationRule, IDialogFlowStep } from '@/types/common';

import InfoTooltip from '@/components/dialogs/accessSetupSteps/InfoTooltip.vue';
import PasswordInputEyeIcons from '@/components/PasswordInputEyeIcons.vue';

const emit = defineEmits<{
    'selectOption': [option: PassphraseOption];
    'passphraseChanged': [passphrase: string];
    'submit': [];
}>();

const form = ref<VForm | null>(null);
const passphraseOption = ref<PassphraseOption>();
const passphrase = ref<string>('');
const isPassphraseVisible = ref<boolean>(false);
const areAdvancedOptionsShown = ref<boolean>(false);

watch(passphraseOption, value => value && emit('selectOption', value));

const bucketsStore = useBucketsStore();

/**
 * Indicates whether the user should be prompted to enter the project passphrase.
 */
const isPromptForPassphrase = computed<boolean>(() => bucketsStore.state.promptForPassphrase);

/**
 * Indicates whether an option in the Advanced menu has been selected.
 */
const isAdvancedOptionSelected = computed<boolean>(() => {
    return passphraseOption.value === PassphraseOption.EnterNewPassphrase
        || passphraseOption.value === PassphraseOption.GenerateNewPassphrase;
});

const passphraseRules = computed<ValidationRule<string>[]>(() => {
    const required = passphraseOption.value === PassphraseOption.SetMyProjectPassphrase;
    return [ v => !required || !!v || 'Required' ];
});

defineExpose<IDialogFlowStep>({
    validate: () => {
        const passphraseRequired = passphraseOption.value === PassphraseOption.SetMyProjectPassphrase;
        if (!passphraseRequired) return true;

        form.value?.validate();
        return !!form.value?.isValid && !!passphrase.value;
    },
    onEnter: () => {
        if (passphraseOption.value) return;
        passphraseOption.value = isPromptForPassphrase.value ?
            PassphraseOption.SetMyProjectPassphrase :
            PassphraseOption.UseExistingPassphrase;
    },
    onExit: () => {
        if (passphraseOption.value === PassphraseOption.UseExistingPassphrase) {
            emit('passphraseChanged', bucketsStore.state.passphrase);
        } else {
            emit('passphraseChanged', passphrase.value);
        }
    },
});
</script>

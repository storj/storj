// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form ref="form" class="pa-8 pb-4">
        <v-row>
            <v-col cols="12">
                <p class="text-subtitle-2 font-weight-bold mb-2">Encryption Passphrase</p>
                <v-radio-group v-model="passphraseOption" hide-details="auto">
                    <template v-if="isPromptForPassphrase">
                        <v-radio v-if="isPromptForPassphrase" label="Enter your project passphrase" :value="PassphraseOption.SetMyProjectPassphrase">
                            <template #label>
                                Enter my project passphrase
                                <info-tooltip>
                                    You will enter your encryption passphrase on the next step.
                                    Make sure it's the same one you use for this project.
                                    This will allow you to manage existing data you have uploaded with the same passphrase.
                                </info-tooltip>
                            </template>
                        </v-radio>
                        <v-text-field
                            v-model="passphrase"
                            class="mt-3"
                            variant="outlined"
                            label="Enter Encryption Passphrase"
                            :append-inner-icon="isPassphraseVisible ? 'mdi-eye-off' : 'mdi-eye'"
                            :type="isPassphraseVisible ? 'text' : 'password'"
                            :rules="passphraseRules"
                            @click:append-inner="isPassphraseVisible = !isPassphraseVisible"
                        />
                        <v-divider class="my-4" />
                    </template>
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
                    <v-alert class="mb-4" type="warning" variant="tonal" rounded="xlg">
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
    VDivider,
} from 'vuetify/components';

import { PassphraseOption } from '@/types/createAccessGrant';
import { CreateAccessStepComponent } from '@poc/types/createAccessGrant';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { RequiredRule, ValidationRule } from '@poc/types/common';

import InfoTooltip from '@poc/components/dialogs/createAccessSteps/InfoTooltip.vue';

const emit = defineEmits<{
    'selectOption': [option: PassphraseOption];
    'passphraseChanged': [passphrase: string];
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

defineExpose<CreateAccessStepComponent>({
    title: 'Access Encryption',
    validate: () => {
        form.value?.validate();
        const passphraseRequired = passphraseOption.value === PassphraseOption.SetMyProjectPassphrase;
        return !!form.value?.isValid && (!passphraseRequired || !!passphrase.value);
    },
    onEnter: () => {
        if (passphraseOption.value) return;
        passphraseOption.value = isPromptForPassphrase.value ?
            PassphraseOption.SetMyProjectPassphrase :
            PassphraseOption.UseExistingPassphrase;
    },
    onExit: () => {
        switch (passphraseOption.value) {
        case PassphraseOption.UseExistingPassphrase:
            emit('passphraseChanged', bucketsStore.state.passphrase);
            break;
        case PassphraseOption.SetMyProjectPassphrase:
            emit('passphraseChanged', passphrase.value);
            break;
        }
    },
});
</script>

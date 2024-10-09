// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form ref="form" class="pa-6">
        <v-row>
            <v-col v-if="$slots.default" cols="12">
                <slot />
            </v-col>
            <save-buttons :name="name" :items="[ passphrase ]" type="passphrase" />
            <v-col cols="12">
                <text-output-area
                    label="Encryption Passphrase"
                    :value="passphrase"
                    :tooltip-disabled="isTooltipDisabled"
                    show-copy
                />
                <v-checkbox
                    density="compact"
                    color="primary"
                    label="I saved my encryption passphrase."
                    :hide-details="false"
                    :rules="[ RequiredRule ]"
                    class="mt-4 mb-n7"
                />
            </v-col>
        </v-row>
    </v-form>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { generateMnemonic } from 'bip39-english';
import { VForm, VRow, VCol, VCheckbox } from 'vuetify/components';

import { RequiredRule, IDialogFlowStep } from '@/types/common';

import TextOutputArea from '@/components/dialogs/accessSetupSteps/TextOutputArea.vue';
import SaveButtons from '@/components/dialogs/commonPassphraseSteps/SaveButtons.vue';

defineProps<{
    name: string;
}>();

const emit = defineEmits<{
    'passphraseChanged': [passphrase: string];
}>();

const form = ref<VForm | null>(null);
const isTooltipDisabled = ref<boolean>(false);

const passphrase: string = generateMnemonic();

defineExpose<IDialogFlowStep>({
    onEnter: () => {
        emit('passphraseChanged', passphrase);
        isTooltipDisabled.value = false;
    },
    onExit: () => isTooltipDisabled.value = true,
    validate: () => {
        form.value?.validate();
        return !!form.value?.isValid;
    },
});
</script>

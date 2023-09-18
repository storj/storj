// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form ref="form" class="pa-8">
        <v-row>
            <v-col v-if="$slots.default" cols="12">
                <slot />
            </v-col>
            <save-buttons :name="name" :items="[ passphrase ]" type="passphrase" />
            <v-divider class="my-3" />

            <v-col cols="12">
                <text-output-area
                    label="Encryption Passphrase"
                    :value="passphrase"
                    center-text
                    :tooltip-disabled="isTooltipDisabled"
                    show-copy
                />
            </v-col>
            <v-col cols="12">
                <v-checkbox
                    density="compact"
                    color="primary"
                    label="Yes, I saved my encryption passphrase."
                    :hide-details="false"
                    :rules="[ RequiredRule ]"
                />
            </v-col>
        </v-row>
    </v-form>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { generateMnemonic } from 'bip39-english';
import { VForm, VRow, VCol, VCheckbox, VDivider } from 'vuetify/components';

import { RequiredRule, DialogStepComponent } from '@poc/types/common';

import TextOutputArea from '@poc/components/dialogs/createAccessSteps/TextOutputArea.vue';
import SaveButtons from '@poc/components/dialogs/commonPassphraseSteps/SaveButtons.vue';

import Icon from '@/../static/images/accessGrants/newCreateFlow/passphraseGenerated.svg';

const props = defineProps<{
    name: string;
}>();

const emit = defineEmits<{
    'passphraseChanged': [passphrase: string];
}>();

const form = ref<VForm | null>(null);
const isTooltipDisabled = ref<boolean>(false);

const passphrase: string = generateMnemonic();

defineExpose<DialogStepComponent>({
    title: 'Passphrase Generated',
    iconSrc: Icon,
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

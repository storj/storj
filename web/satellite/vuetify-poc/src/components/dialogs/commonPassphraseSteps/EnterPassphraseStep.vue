// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form ref="form" class="pa-8" @submit.prevent>
        <v-row>
            <v-col v-if="$slots.default" cols="12">
                <slot />
            </v-col>

            <v-col cols="12">
                <v-text-field
                    v-model="passphrase"
                    label="Encryption Passphrase"
                    :append-inner-icon="isPassphraseVisible ? 'mdi-eye-off' : 'mdi-eye'"
                    :type="isPassphraseVisible ? 'text' : 'password'"
                    variant="outlined"
                    :hide-details="false"
                    :rules="[ RequiredRule ]"
                    @click:append-inner="isPassphraseVisible = !isPassphraseVisible"
                />
            </v-col>

            <v-col v-if="ackRequired" cols="12">
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
import { ref, watch } from 'vue';
import { VForm, VRow, VCol, VTextField, VCheckbox } from 'vuetify/components';

import { RequiredRule, DialogStepComponent } from '@poc/types/common';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { EdgeCredentials } from '@/types/accessGrants';
import { useNotify } from '@/utils/hooks';

const bucketsStore = useBucketsStore();
const notify = useNotify();

const form = ref<VForm | null>(null);

const passphrase = ref<string>('');
const isPassphraseVisible = ref<boolean>(false);

const props = withDefaults(defineProps<{
    title: string;
    setOnNext: boolean;
    ackRequired: boolean;
}>(), {
    title: 'Enter New Passphrase',
    setOnNext: false,
    ackRequired: false,
});

const emit = defineEmits<{
    'passphraseChanged': [passphrase: string];
}>();

watch(passphrase, value => emit('passphraseChanged', value));

defineExpose<DialogStepComponent>({
    title: props.title,
    validate: () => {
        form.value?.validate();
        return !!form.value?.isValid;
    },
    onExit: to => {
        if (!props.setOnNext || to !== 'next') return;

        bucketsStore.setEdgeCredentials(new EdgeCredentials());
        bucketsStore.setPassphrase(passphrase.value);
        bucketsStore.setPromptForPassphrase(false);

        notify.success('Passphrase switched.');
    },
});
</script>

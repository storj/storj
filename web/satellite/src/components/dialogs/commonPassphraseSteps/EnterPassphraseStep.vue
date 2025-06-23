// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form ref="form" class="pa-6" @submit.prevent="emit('submit')">
        <v-row>
            <v-col v-if="$slots.default" cols="12">
                <slot />
            </v-col>

            <v-col cols="12">
                <v-text-field
                    id="Encryption Passphrase"
                    v-model="passphrase"
                    label="Encryption Passphrase"
                    :type="isPassphraseVisible ? 'text' : 'password'"
                    variant="outlined"
                    :hide-details="false"
                    :rules="[ RequiredRule ]"
                    autofocus
                    required
                    class="mt-2 mb-n4"
                >
                    <template #append-inner>
                        <password-input-eye-icons
                            :is-visible="isPassphraseVisible"
                            type="passphrase"
                            @toggle-visibility="isPassphraseVisible = !isPassphraseVisible"
                        />
                    </template>
                </v-text-field>
                <v-checkbox
                    v-if="ackRequired"
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
import { ref, watch } from 'vue';
import { VForm, VRow, VCol, VTextField, VCheckbox } from 'vuetify/components';

import { RequiredRule, DialogStepComponent } from '@/types/common';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { EdgeCredentials } from '@/types/accessGrants';
import { useNotify } from '@/composables/useNotify';

import PasswordInputEyeIcons from '@/components/PasswordInputEyeIcons.vue';

const bucketsStore = useBucketsStore();
const notify = useNotify();

const form = ref<VForm | null>(null);

const passphrase = ref<string>('');
const isPassphraseVisible = ref<boolean>(false);

const props = withDefaults(defineProps<{
    title?: string;
    setOnNext?: boolean;
    ackRequired?: boolean;
}>(), {
    title: 'Enter New Passphrase',
    setOnNext: false,
    ackRequired: false,
});

const emit = defineEmits<{
    'passphraseChanged': [passphrase: string];
    'submit': [];
}>();

watch(passphrase, value => emit('passphraseChanged', value));

defineExpose<DialogStepComponent>({
    title: props.title,
    validate: () => {
        form.value?.validate();
        return !!form.value?.isValid;
    },
    onEnter: () => emit('passphraseChanged', passphrase.value),
    onExit: to => {
        if (!props.setOnNext || to !== 'next') return;

        bucketsStore.setEdgeCredentials(new EdgeCredentials());
        bucketsStore.setPassphrase(passphrase.value);
        bucketsStore.setPromptForPassphrase(false);

        notify.success('Passphrase switched.');
    },
});
</script>

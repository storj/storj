// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form ref="form" class="pa-8">
        <v-row>
            <v-col cols="12">
                By generating S3 credentials, you are opting in to
                <a class="link" href="https://docs.storj.io/dcs/concepts/encryption-key/design-decision-server-side-encryption/">
                    server-side encryption.
                </a>
            </v-col>
            <v-col cols="12">
                <v-checkbox
                    density="compact"
                    label="I understand, don't show this again."
                    :hide-details="false"
                    :rules="[ RequiredRule ]"
                    @update:model-value="value => LocalData.setServerSideEncryptionModalHidden(value)"
                />
            </v-col>
        </v-row>
    </v-form>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { VForm, VRow, VCol, VCheckbox } from 'vuetify/components';

import { LocalData } from '@/utils/localData';
import { RequiredRule, DialogStepComponent } from '@poc/types/common';

const form = ref<VForm | null>(null);

defineExpose<DialogStepComponent>({
    title: 'Encryption Information',
    validate: () => {
        form.value?.validate();
        return !!form.value?.isValid;
    },
});
</script>
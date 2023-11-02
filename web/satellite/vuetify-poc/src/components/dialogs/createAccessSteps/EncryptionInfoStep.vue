// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row class="pa-4 ma-0">
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
                color="default"
                :hide-details="false"
                :rules="[ RequiredRule ]"
                @update:model-value="value => LocalData.setServerSideEncryptionModalHidden(value)"
            />
        </v-col>
    </v-row>
</template>

<script setup lang="ts">
import { VRow, VCol, VCheckbox } from 'vuetify/components';

import { LocalData } from '@/utils/localData';
import { DialogStepComponent } from '@poc/types/common';

defineExpose<DialogStepComponent>({
    title: 'Encryption Information',
});
</script>
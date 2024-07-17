// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form ref="form" class="pa-7 pb-3" @submit.prevent="() => emit('submit')">
        <v-row>
            <v-col cols="12">
                <p class="mb-3">Enter your domain name (URL)</p>
                <v-text-field
                    v-model="domain"
                    label="Website URL"
                    placeholder="yourdomain.com"
                    variant="outlined"
                    :rules="[RequiredRule, DomainRule]"
                    required
                    autofocus
                />
            </v-col>

            <v-col>
                <p class="mb-3">Select a bucket to share files</p>
                <v-autocomplete
                    v-model="bucket"
                    :items="allBucketNames"
                    variant="outlined"
                    label="Bucket"
                    :rules="[RequiredRule]"
                    required
                />
            </v-col>
        </v-row>
    </v-form>
</template>

<script setup lang="ts">
import { watch, ref, computed } from 'vue';
import { VAutocomplete, VCol, VForm, VRow, VTextField } from 'vuetify/components';

import { DomainRule, IDialogFlowStep, RequiredRule } from '@/types/common';
import { useBucketsStore } from '@/store/modules/bucketsStore';

const bucketsStore = useBucketsStore();

const form = ref<VForm>();
const domain = ref<string>('');
const bucket = ref<string | undefined>(undefined);

const emit = defineEmits<{
    'domainChanged': [domain: string];
    'bucketChanged': [bucket: string | undefined];
    'submit': [];
}>();

/**
 * Returns all bucket names from the store.
 */
const allBucketNames = computed<string[]>(() => bucketsStore.state.allBucketNames);

watch(domain, value => emit('domainChanged', value));
watch(bucket, value => emit('bucketChanged', value));

defineExpose<IDialogFlowStep>({
    validate: () => {
        form.value?.validate();
        return !!form.value?.isValid;
    },
});
</script>

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form ref="form" class="pa-8" @submit.prevent>
        <v-row>
            <v-col cols="12">
                <v-text-field
                    v-model="name"
                    label="Access Name"
                    placeholder="Enter name for this access"
                    variant="outlined"
                    color="default"
                    autofocus
                    :hide-details="false"
                    :rules="nameRules"
                />
            </v-col>

            <v-col cols="12">
                <h4 class="mb-2">Type</h4>
                <v-input v-model="types" :rules="[ RequiredRule ]" :hide-details="false">
                    <div>
                        <v-checkbox
                            v-for="accessType in typeOrder"
                            :key="accessType"
                            v-model="typeInfos[accessType].model.value"
                            color="primary"
                            density="compact"
                            :hide-details="true"
                        >
                            <template #label>
                                <span class="ml-2">{{ typeInfos[accessType].name }}</span>
                                <info-tooltip>
                                    {{ typeInfos[accessType].description }}
                                    <a class="text-surface" :href="ACCESS_TYPE_LINKS[accessType]" target="_blank">
                                        Learn more
                                    </a>
                                </info-tooltip>
                            </template>
                        </v-checkbox>
                    </div>
                </v-input>
            </v-col>
        </v-row>
    </v-form>
</template>

<script setup lang="ts">
import { computed, ref, watch, WritableComputedRef } from 'vue';
import { VForm, VRow, VCol, VTextField, VCheckbox, VInput } from 'vuetify/components';

import { AccessType } from '@/types/createAccessGrant';
import { ACCESS_TYPE_LINKS, CreateAccessStepComponent } from '@poc/types/createAccessGrant';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { RequiredRule, ValidationRule } from '@poc/types/common';

import InfoTooltip from '@poc/components/dialogs/createAccessSteps/InfoTooltip.vue';

class AccessTypeInfo {
    public model: WritableComputedRef<boolean>;
    constructor(
        public accessType: AccessType,
        public name: string,
        public description: string,
        public exclusive: boolean = false,
    ) {
        this.model = computed<boolean>({
            get: () => types.value.includes(accessType),
            set: (checked: boolean) => {
                if (!checked) {
                    types.value = types.value.filter(iterType => iterType !== accessType);
                    return;
                }
                if (typeInfos[this.accessType].exclusive) {
                    types.value = [this.accessType];
                    return;
                }
                types.value = [...types.value.filter(iter => !typeInfos[iter].exclusive), accessType];
            },
        });
    }
}

const typeInfos: Record<AccessType, AccessTypeInfo> = {
    [AccessType.AccessGrant]: new AccessTypeInfo(
        AccessType.AccessGrant,
        'Access Grant',
        'Keys to upload, delete, and view your data.',
    ),
    [AccessType.S3]: new AccessTypeInfo(
        AccessType.S3,
        'S3 Credentials',
        'Generates access key, secret key, and endpoint to use in your S3 supported application.',
    ),
    [AccessType.APIKey]: new AccessTypeInfo(
        AccessType.APIKey,
        'CLI Access',
        'Create an access grant to run in the command line.',
        true,
    ),
};

const typeOrder: AccessType[] = [
    AccessType.AccessGrant,
    AccessType.S3,
    AccessType.APIKey,
];

const form = ref<VForm | null>(null);

const name = ref<string>('');
const types = ref<AccessType[]>([]);

watch(name, value => emit('nameChanged', value));
watch(types, value => emit('typesChanged', value.slice()), { deep: true });

const emit = defineEmits<{
    'nameChanged': [name: string];
    'typesChanged': [types: AccessType[]];
}>();

const agStore = useAccessGrantsStore();

const nameRules: ValidationRule<string>[] = [
    RequiredRule,
    v => !agStore.state.allAGNames.includes(v) || 'This name is already in use',
];

defineExpose<CreateAccessStepComponent>({
    title: 'Create New Access',
    validate: () => {
        form.value?.validate();
        return !!form.value?.isValid;
    },
});
</script>

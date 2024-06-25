// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form ref="form" class="pa-6 pb-4" @submit.prevent="emit('submit')">
        <v-row>
            <v-col cols="12">
                <p class="text-subtitle-2 font-weight-bold mb-5">Enter Access Name</p>
                <v-text-field
                    v-model="name"
                    label="Access Name"
                    variant="outlined"
                    autofocus
                    :hide-details="false"
                    :rules="nameRules"
                    maxlength="100"
                    class="mb-n2"
                    required
                />
            </v-col>
            <v-col cols="12">
                <p class="text-subtitle-2 font-weight-bold mb-2">Select Access Type</p>
                <v-input v-model="types" :rules="[ RequiredRule ]" :hide-details="false" class="mb-n2">
                    <div>
                        <v-checkbox
                            v-for="accessType in typeOrder"
                            :key="accessType"
                            v-model="typeInfos[accessType].model.value"
                            density="compact"
                            hide-details
                        >
                            <template #label>
                                <span>{{ typeInfos[accessType].name }}</span>
                                <info-tooltip>
                                    {{ typeInfos[accessType].description }}
                                    <a class="link" :href="ACCESS_TYPE_LINKS[accessType]" target="_blank">
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

import { AccessType, Exposed, ACCESS_TYPE_LINKS } from '@/types/createAccessGrant';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { RequiredRule, ValidationRule, DialogStepComponent } from '@/types/common';
import { useProjectsStore } from '@/store/modules/projectsStore';

import InfoTooltip from '@/components/dialogs/createAccessSteps/InfoTooltip.vue';

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

const projectStore = useProjectsStore();

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
        'API Key',
        'Create an access grant to run in the command line.',
        true,
    ),
};

const typeOrder = computed<AccessType[]> (() => {
    const order = [ AccessType.AccessGrant, AccessType.S3 ];
    if (!projectStore.state.selectedProjectConfig.passphrase) {
        order.push(AccessType.APIKey);
    }
    return order;
});

const form = ref<VForm | null>(null);

const name = ref<string>('');
const types = ref<AccessType[]>([]);

watch(name, value => emit('nameChanged', value));
watch(types, value => emit('typesChanged', value.slice()), { deep: true });

const emit = defineEmits<{
    'nameChanged': [name: string];
    'typesChanged': [types: AccessType[]];
    'submit': [];
}>();

const agStore = useAccessGrantsStore();

const nameRules: ValidationRule<string>[] = [
    RequiredRule,
    v => !agStore.state.allAGNames.includes(v) || 'This name is already in use',
];

defineExpose<DialogStepComponent | Exposed>({
    title: 'New Access Key',
    validate: () => {
        form.value?.validate();
        return !!form.value?.isValid;
    },
    setName: (newName: string) => name.value = newName,
    setTypes: (newTypes: AccessType[]) => types.value = newTypes,
});
</script>

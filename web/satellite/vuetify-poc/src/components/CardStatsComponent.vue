// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card :subtitle="subtitle" variant="flat" :border="true" rounded="xlg" :to="to">
        <template #title>
            <v-card-title class="d-flex align-center">
                <component :is="iconComponent" v-if="icon" v-bind="iconProps" class="mr-2" width="16" height="16" />
                {{ title }}
            </v-card-title>
        </template>
        <v-card-text>
            <v-chip rounded color="green" variant="outlined" class="font-weight-bold">{{ data }}</v-chip>
        </v-card-text>
    </v-card>
</template>

<script setup lang="ts">
import { computed, Component } from 'vue';
import { VCard, VCardText, VChip, VCardTitle } from 'vuetify/components';

import IconFile from '@poc/components/icons/IconFile.vue';
import IconGlobe from '@poc/components/icons/IconGlobe.vue';
import IconBucket from '@poc/components/icons/IconBucket.vue';
import IconAccess from '@poc/components/icons/IconAccess.vue';
import IconTeam from '@poc/components/icons/IconTeam.vue';
import IconCard from '@poc/components/icons/IconCard.vue';

const props = defineProps<{
    title: string;
    subtitle: string;
    data: string;
    to: string;
    icon?: keyof typeof iconComponents;
}>();

const iconComponents = {
    file: IconFile,
    globe: IconGlobe,
    bucket: IconBucket,
    access: IconAccess,
    team: IconTeam,
    card: IconCard,
};

const iconComponent = computed<Component | null>(() => props.icon ? iconComponents[props.icon] : null);
const iconProps = computed<object | null>(() => iconComponent.value?.['props']?.['bold'] ? { bold: true } : null);
</script>

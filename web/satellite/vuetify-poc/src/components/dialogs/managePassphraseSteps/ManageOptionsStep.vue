// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pa-4">
        <v-sheet v-for="item in items" :key="item.next" class="bg-surface-emphasis ma-4" border rounded>
            <v-list-item class="py-2 px-5" link @click="emit('optionClick', item.next)">
                <div class="d-flex flex-row align-center">
                    <component :is="item.icon" />
                    <div class="mx-4">
                        <p class="font-weight-bold mb-1">{{ item.title }}</p>
                        <p class="text-caption">{{ item.subtitle }}</p>
                    </div>
                    <v-spacer />
                    <v-icon size="32" icon="mdi-chevron-right" color="primary" />
                </div>
            </v-list-item>
        </v-sheet>
    </div>
</template>

<script setup lang="ts">
import { Component } from 'vue';
import { VSheet, VListItem, VSpacer, VIcon } from 'vuetify/components';

import { ManageProjectPassphraseStep } from '@poc/types/managePassphrase';
import { DialogStepComponent } from '@poc/types/common';

import IconCirclePlus from '@poc/components/icons/IconCirclePlus.vue';
import IconSwitch from '@poc/components/icons/IconSwitch.vue';
import IconLock from '@poc/components/icons/IconLock.vue';

type Item = {
    icon: Component;
    title: string;
    subtitle: string;
    next: ManageProjectPassphraseStep;
};

const items: Item[] = [
    {
        icon: IconCirclePlus,
        title: 'Create a new passphrase',
        subtitle: 'Allows you to upload data with a different passphrase.',
        next: ManageProjectPassphraseStep.Create,
    }, {
        icon: IconSwitch,
        title: 'Switch active passphrase',
        subtitle: 'View and upload data using another passphrase.',
        next: ManageProjectPassphraseStep.Switch,
    }, {
        icon: IconLock,
        title: 'Clear saved passphrase',
        subtitle: 'Lock your data and clear passphrase from this session.',
        next: ManageProjectPassphraseStep.Clear,
    },
];

const emit = defineEmits<{
    'optionClick': [option: ManageProjectPassphraseStep];
}>();

defineExpose<DialogStepComponent>({
    title: 'Manage Passphrase',
});
</script>

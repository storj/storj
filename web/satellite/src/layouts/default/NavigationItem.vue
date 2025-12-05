// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-list-item link lines="one" :to="to" class="my-1" tabindex="0" active-class="text-primary" @click="onClick" @keydown.space.prevent="onClick">
        <template #prepend>
            <slot name="prepend" />
        </template>
        <v-list-item-title class="ml-4">{{ title }}</v-list-item-title>
        <v-list-item-subtitle v-if="subtitle" class="ml-4">{{ subtitle }}</v-list-item-subtitle>
        <template #append>
            <slot name="append" />
        </template>
    </v-list-item>
</template>

<script setup lang="ts">
import { VListItem, VListItemTitle, VListItemSubtitle } from 'vuetify/components';
import { useDisplay } from 'vuetify';
import { useRouter } from 'vue-router';

import { useAppStore } from '@/store/modules/appStore';

const { mdAndDown } = useDisplay();
const router = useRouter();

const appStore = useAppStore();

const props = defineProps<{
    title: string;
    subtitle?: string;
    to?: string;
}>();

/**
 * Conditionally closes the navigation drawer and tracks page visit.
 */
function onClick(e: MouseEvent | KeyboardEvent): void {
    if (!props.to) return;

    const next = router.resolve(props.to).path;
    if (next === router.currentRoute.value.path) return;

    if (mdAndDown.value) appStore.toggleNavigationDrawer(false);

    // Vuetify handles navigation via click or pressing the Enter key.
    // We must handle the space key ourselves.
    if ('key' in e && e.key === ' ') router.push(props.to);
}
</script>

// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <managed-passphrase-opt-in-selector
            :selected-mode="selectedMode"
            :loading="loading"
            @back="emit('back')"
            @mode-chosen="onModeChosen"
        />
    </v-container>
</template>

<script setup lang="ts">
import { VContainer } from 'vuetify/components';
import { ref } from 'vue';

import { useProjectsStore } from '@/store/modules/projectsStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { ManagePassphraseMode } from '@/types/projects';

import ManagedPassphraseOptInSelector from '@/components/dialogs/accountSetupSteps/ManagedPassphraseOptInSelector.vue';

const projectsStore = useProjectsStore();
const userStore = useUsersStore();

defineProps<{
    loading: boolean,
}>();

const emit = defineEmits<{
    next: [];
    back: [];
}>();

const selectedMode = ref<ManagePassphraseMode>();

function onModeChosen(mode: ManagePassphraseMode) {
    selectedMode.value = mode;
    emit('next');
}

async function setup() {
    await projectsStore.createDefaultProject(userStore.state.user.id, selectedMode.value === 'auto');
}

defineExpose({
    validate: () => !!selectedMode.value,
    setup,
});
</script>

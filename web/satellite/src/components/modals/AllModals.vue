// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <CreateProjectPromptModal v-if="isCreateProjectPromptModal" />
        <AddPaymentMethodModal v-if="isAddPMModal" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import CreateProjectPromptModal from '@/components/modals/CreateProjectPromptModal.vue';
import AddPaymentMethodModal from "@/components/modals/AddPaymentMethodModal.vue";

// @vue/component
@Component({
    components: {
        CreateProjectPromptModal,
        AddPaymentMethodModal,
    },
})
export default class AllModals extends Vue {
    /**
     * Indicates if create project prompt modal is shown.
     */
    public get isCreateProjectPromptModal(): boolean {
        return this.$store.state.appStateModule.appState.isCreateProjectPromptModalShown;
    }

    /**
     * Indicates if add payment method modal is shown.
     */
    public get isAddPMModal(): boolean {
        return this.$store.state.appStateModule.appState.isAddPMModalShown;
    }
}
</script>

// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <CreateProjectPromptModal v-if="isCreateProjectPromptModal" />
        <CreateProjectModal v-if="isCreateProjectModal" />
        <AddPaymentMethodModal v-if="isAddPMModal" />
        <OpenBucketModal v-if="isOpenBucketModal" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import CreateProjectPromptModal from '@/components/modals/CreateProjectPromptModal.vue';
import CreateProjectModal from '@/components/modals/CreateProjectModal.vue'
import AddPaymentMethodModal from "@/components/modals/AddPaymentMethodModal.vue";
import OpenBucketModal from "@/components/modals/OpenBucketModal.vue";

// @vue/component
@Component({
    components: {
        CreateProjectPromptModal,
        CreateProjectModal,
        AddPaymentMethodModal,
        OpenBucketModal,
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
     * Indicates if create project prompt modal is shown.
     */
    public get isCreateProjectModal(): boolean {
        return this.$store.state.appStateModule.appState.isCreateProjectModalShown;
    }

    /**
     * Indicates if add payment method modal is shown.
     */
    public get isAddPMModal(): boolean {
        return this.$store.state.appStateModule.appState.isAddPMModalShown;
    }

    /**
     * Indicates if open bucket modal is shown.
     */
    public get isOpenBucketModal(): boolean {
        return this.$store.state.appStateModule.appState.isOpenBucketModalShown;
    }
}
</script>

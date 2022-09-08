// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <CreateProjectPromptModal v-if="isCreateProjectPromptModal" />
        <CreateProjectModal v-if="isCreateProjectModal" />
        <AddPaymentMethodModal v-if="isAddPMModal" />
        <OpenBucketModal v-if="isOpenBucketModal" />
        <MFARecoveryCodesModal v-if="isMFARecoveryModal" />
        <EnableMFAModal v-if="isEnableMFAModal" />
        <DisableMFAModal v-if="isDisableMFAModal" />
        <EditProfileModal v-if="isEditProfileModal" />
        <ChangePasswordModal v-if="isChangePasswordModal" />
        <AddTeamMemberModal v-if="isAddTeamMembersModal" />
        <AddTokenFundsModal v-if="isAddTokenFundsModal" />
        <ShareBucketModal v-if="isShareBucketModal" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import CreateProjectPromptModal from '@/components/modals/CreateProjectPromptModal.vue';
import CreateProjectModal from '@/components/modals/CreateProjectModal.vue';
import AddPaymentMethodModal from '@/components/modals/AddPaymentMethodModal.vue';
import OpenBucketModal from '@/components/modals/OpenBucketModal.vue';
import MFARecoveryCodesModal from '@/components/modals/MFARecoveryCodesModal.vue';
import EnableMFAModal from '@/components/modals/EnableMFAModal.vue';
import DisableMFAModal from '@/components/modals/DisableMFAModal.vue';
import EditProfileModal from '@/components/modals/EditProfileModal.vue';
import ChangePasswordModal from '@/components/modals/ChangePasswordModal.vue';
import AddTeamMemberModal from '@/components/modals/AddTeamMemberModal.vue';
import AddTokenFundsModal from '@/components/modals/AddTokenFundsModal.vue';
import ShareBucketModal from '@/components/modals/ShareBucketModal.vue';

// @vue/component
@Component({
    components: {
        CreateProjectPromptModal,
        CreateProjectModal,
        AddPaymentMethodModal,
        OpenBucketModal,
        MFARecoveryCodesModal,
        EnableMFAModal,
        DisableMFAModal,
        EditProfileModal,
        ChangePasswordModal,
        AddTeamMemberModal,
        AddTokenFundsModal,
        ShareBucketModal,
    },
})
export default class AllModals extends Vue {
    // TODO: add active modal indicator.
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

    /**
     * Indicates if MFA recovery modal is shown.
     */
    public get isMFARecoveryModal(): boolean {
        return this.$store.state.appStateModule.appState.isMFARecoveryModalShown;
    }

    /**
     * Indicates if enable MFA modal is shown.
     */
    public get isEnableMFAModal(): boolean {
        return this.$store.state.appStateModule.appState.isEnableMFAModalShown;
    }

    /**
     * Indicates if disable MFA modal is shown.
     */
    public get isDisableMFAModal(): boolean {
        return this.$store.state.appStateModule.appState.isDisableMFAModalShown;
    }

    /**
     * Indicates if edit profile modal is shown.
     */
    public get isEditProfileModal(): boolean {
        return this.$store.state.appStateModule.appState.isEditProfileModalShown;
    }

    /**
     * Indicates if change password modal is shown.
     */
    public get isChangePasswordModal(): boolean {
        return this.$store.state.appStateModule.appState.isChangePasswordModalShown;
    }

    /**
     * Indicates if add team members modal is shown.
     */
    public get isAddTeamMembersModal(): boolean {
        return this.$store.state.appStateModule.appState.isAddTeamMembersModalShown;
    }

    /**
     * Indicates if add token funds modal is shown.
     */
    public get isAddTokenFundsModal(): boolean {
        return this.$store.state.appStateModule.appState.isAddTokenFundsModalShown;
    }

    /**
     * Indicates if share bucket modal is shown.
     */
    public get isShareBucketModal(): boolean {
        return this.$store.state.appStateModule.appState.isShareBucketModalShown;
    }
}
</script>

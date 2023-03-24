// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="activeModal">
        <component :is="activeModal" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import CreateProjectPromptModal from '@/components/modals/CreateProjectPromptModal.vue';
import CreateProjectModal from '@/components/modals/CreateProjectModal.vue';
import NewCreateProjectModal from '@/components/modals/NewCreateProjectModal.vue';
import CreateBucketModal from '@/components/modals/CreateBucketModal.vue';
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
import ObjectDetailsModal from '@/components/modals/ObjectDetailsModal.vue';
import NewFolderModal from '@/components/modals/NewFolderModal.vue';
import ShareObjectModal from '@/components/modals/ShareObjectModal.vue';
import DeleteBucketModal from '@/components/modals/DeleteBucketModal.vue';
import AddCouponCodeModal from '@/components/modals/AddCouponCodeModal.vue';
import NewBillingAddCouponCodeModal from '@/components/modals/NewBillingAddCouponCodeModal.vue';
import CreateProjectPassphraseModal from '@/components/modals/createProjectPassphrase/CreateProjectPassphraseModal.vue';
import ManageProjectPassphraseModal from '@/components/modals/manageProjectPassphrase/ManageProjectPassphraseModal.vue';
import EnterPassphraseModal from '@/components/modals/EnterPassphraseModal.vue';
import PricingPlanModal from '@/components/modals/PricingPlanModal.vue';

// @vue/component
@Component({
    components: {
        EnterPassphraseModal,
        ManageProjectPassphraseModal,
        CreateProjectPassphraseModal,
        DeleteBucketModal,
        CreateProjectPromptModal,
        CreateProjectModal,
        NewCreateProjectModal,
        CreateBucketModal,
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
        ShareObjectModal,
        ObjectDetailsModal,
        NewFolderModal,
        AddCouponCodeModal,
        NewBillingAddCouponCodeModal,
        PricingPlanModal,
    },
})
export default class AllModals extends Vue {
    // TODO: add active modal indicator.

    /**
   * Indicates the current active modal.
   */
    public get activeModal(): unknown | null {
        // modal could be of VueConstructor type or Object (for composition api components).
        return this.$store.state.appStateModule.viewsState.activeModal;
    }
}
</script>

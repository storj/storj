// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import AddTeamMemberModal from '@/components/modals/AddTeamMemberModal.vue';
import EditProfileModal from '@/components/modals/EditProfileModal.vue';
import ChangePasswordModal from '@/components/modals/ChangePasswordModal.vue';
import CreateProjectModal from '@/components/modals/CreateProjectModal.vue';
import OpenBucketModal from '@/components/modals/OpenBucketModal.vue';
import MFARecoveryCodesModal from '@/components/modals/MFARecoveryCodesModal.vue';
import EnableMFAModal from '@/components/modals/EnableMFAModal.vue';
import DisableMFAModal from '@/components/modals/DisableMFAModal.vue';
import AddTokenFundsModal from '@/components/modals/AddTokenFundsModal.vue';
import ShareBucketModal from '@/components/modals/ShareBucketModal.vue';
import ShareObjectModal from '@/components/modals/ShareObjectModal.vue';
import DeleteBucketModal from '@/components/modals/DeleteBucketModal.vue';
import CreateBucketModal from '@/components/modals/CreateBucketModal.vue';
import NewFolderModal from '@/components/modals/NewFolderModal.vue';
import CreateProjectPassphraseModal
    from '@/components/modals/createProjectPassphrase/CreateProjectPassphraseModal.vue';
import ManageProjectPassphraseModal
    from '@/components/modals/manageProjectPassphrase/ManageProjectPassphraseModal.vue';
import AddCouponCodeModal from '@/components/modals/AddCouponCodeModal.vue';
import NewBillingAddCouponCodeModal
    from '@/components/modals/NewBillingAddCouponCodeModal.vue';
import CreateProjectPromptModal from '@/components/modals/CreateProjectPromptModal.vue';
import UploadCancelPopup from '@/components/objects/UploadCancelPopup.vue';
import ObjectDetailsModal from '@/components/modals/ObjectDetailsModal.vue';
import EnterPassphraseModal from '@/components/modals/EnterPassphraseModal.vue';
import PricingPlanModal from '@/components/modals/PricingPlanModal.vue';
import NewCreateProjectModal from '@/components/modals/NewCreateProjectModal.vue';
import EditSessionTimeoutModal from '@/components/modals/EditSessionTimeoutModal.vue';
import UpgradeAccountModal from '@/components/modals/upgradeAccountFlow/UpgradeAccountModal.vue';

export const APP_STATE_DROPDOWNS = {
    ACCOUNT: 'isAccountDropdownShown',
    ALL_DASH_ACCOUNT: 'allProjectsDashboardAccount',
    SELECT_PROJECT: 'isSelectProjectDropdownShown',
    RESOURCES: 'isResourcesDropdownShown',
    QUICK_START: 'isQuickStartDropdownShown',
    FREE_CREDITS: 'isFreeCreditsDropdownShown',
    AVAILABLE_BALANCE: 'isAvailableBalanceDropdownShown',
    PERIODS: 'isPeriodsDropdownShown',
    BUCKET_NAMES: 'isBucketNamesDropdownShown',
    AG_DATE_PICKER: 'isAGDatePickerShown',
    CHART_DATE_PICKER: 'isChartsDatePickerShown',
    PERMISSIONS: 'isPermissionsDropdownShown',
    PAYMENT_SELECTION: 'isPaymentSelectionShown',
    TIMEOUT_SELECTOR: 'timeoutSelector',
};

enum Modals {
    ADD_TEAM_MEMBER = 'addTeamMember',
    EDIT_PROFILE = 'editProfile',
    CHANGE_PASSWORD = 'changePassword',
    CREATE_PROJECT = 'createProject',
    OPEN_BUCKET = 'openBucket',
    MFA_RECOVERY = 'mfaRecovery',
    ENABLE_MFA = 'enableMFA',
    DISABLE_MFA = 'disableMFA',
    ADD_TOKEN_FUNDS = 'addTokenFunds',
    SHARE_BUCKET = 'shareBucket',
    SHARE_OBJECT = 'shareObject',
    DELETE_BUCKET = 'deleteBucket',
    CREATE_BUCKET = 'createBucket',
    NEW_FOLDER = 'newFolder',
    CREATE_PROJECT_PASSPHRASE = 'createProjectPassphrase',
    MANAGE_PROJECT_PASSPHRASE = 'manageProjectPassphrase',
    ADD_COUPON = 'addCoupon',
    NEW_BILLING_ADD_COUPON = 'newBillingAddCoupon',
    CREATE_PROJECT_PROMPT = 'createProjectPrompt',
    UPLOAD_CANCEL_POPUP = 'uploadCancelPopup',
    OBJECT_DETAILS = 'objectDetails',
    ENTER_PASSPHRASE = 'enterPassphrase',
    PRICING_PLAN = 'pricingPlan',
    NEW_CREATE_PROJECT = 'newCreateProject',
    EDIT_SESSION_TIMEOUT = 'editSessionTimeout',
    UPGRADE_ACCOUNT = 'upgradeAccount',
}

// modals could be of VueConstructor type or Object (for composition api components).
export const MODALS: Record<Modals, unknown> = {
    [Modals.ADD_TEAM_MEMBER]: AddTeamMemberModal,
    [Modals.EDIT_PROFILE]: EditProfileModal,
    [Modals.CHANGE_PASSWORD]: ChangePasswordModal,
    [Modals.CREATE_PROJECT]: CreateProjectModal,
    [Modals.OPEN_BUCKET]: OpenBucketModal,
    [Modals.MFA_RECOVERY]: MFARecoveryCodesModal,
    [Modals.ENABLE_MFA]: EnableMFAModal,
    [Modals.DISABLE_MFA]: DisableMFAModal,
    [Modals.ADD_TOKEN_FUNDS]: AddTokenFundsModal,
    [Modals.SHARE_BUCKET]: ShareBucketModal,
    [Modals.SHARE_OBJECT]: ShareObjectModal,
    [Modals.DELETE_BUCKET]: DeleteBucketModal,
    [Modals.CREATE_BUCKET]: CreateBucketModal,
    [Modals.NEW_FOLDER]: NewFolderModal,
    [Modals.CREATE_PROJECT_PASSPHRASE]: CreateProjectPassphraseModal,
    [Modals.MANAGE_PROJECT_PASSPHRASE]: ManageProjectPassphraseModal,
    [Modals.ADD_COUPON]: AddCouponCodeModal,
    [Modals.NEW_BILLING_ADD_COUPON]: NewBillingAddCouponCodeModal,
    [Modals.CREATE_PROJECT_PROMPT]: CreateProjectPromptModal,
    [Modals.UPLOAD_CANCEL_POPUP]: UploadCancelPopup,
    [Modals.OBJECT_DETAILS]: ObjectDetailsModal,
    [Modals.ENTER_PASSPHRASE]: EnterPassphraseModal,
    [Modals.PRICING_PLAN]: PricingPlanModal,
    [Modals.NEW_CREATE_PROJECT]: NewCreateProjectModal,
    [Modals.EDIT_SESSION_TIMEOUT]: EditSessionTimeoutModal,
    [Modals.UPGRADE_ACCOUNT]: UpgradeAccountModal,
};

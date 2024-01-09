// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { Component } from 'vue';

import AddTeamMemberModal from '@/components/modals/AddTeamMemberModal.vue';
import RemoveTeamMemberModal from '@/components/modals/RemoveProjectMemberModal.vue';
import EditProfileModal from '@/components/modals/EditProfileModal.vue';
import ChangePasswordModal from '@/components/modals/ChangePasswordModal.vue';
import ChangeProjectLimitModal from '@/components/modals/ChangeProjectLimitModal.vue';
import CreateProjectModal from '@/components/modals/CreateProjectModal.vue';
import EnterBucketPassphraseModal from '@/components/modals/EnterBucketPassphraseModal.vue';
import MFARecoveryCodesModal from '@/components/modals/MFARecoveryCodesModal.vue';
import EnableMFAModal from '@/components/modals/EnableMFAModal.vue';
import DisableMFAModal from '@/components/modals/DisableMFAModal.vue';
import AddTokenFundsModal from '@/components/modals/AddTokenFundsModal.vue';
import ShareModal from '@/components/modals/ShareModal.vue';
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
import DeleteAccessGrantModal from '@/components/modals/DeleteAccessGrantModal.vue';
import SkipPassphraseModal from '@/components/modals/SkipPassphraseModal.vue';
import JoinProjectModal from '@/components/modals/JoinProjectModal.vue';
import RequestProjectLimitModal from '@/components/modals/RequestProjectLimitModal.vue';
import DetailedUsageReportModal from '@/components/modals/DetailedUsageReportModal.vue';

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
    PAGE_SIZE_SELECTOR: 'pageSizeSelector',
    SIZE_MEASUREMENT_SELECTOR: 'sizeMeasurementSelector',
    REQUESTED_SIZE_MEASUREMENT_SELECTOR: 'requestedSizeMeasurementSelector',
};

enum Modals {
    ADD_TEAM_MEMBER = 'addTeamMember',
    REMOVE_TEAM_MEMBER = 'removeTeamMember',
    EDIT_PROFILE = 'editProfile',
    CHANGE_PASSWORD = 'changePassword',
    CREATE_PROJECT = 'createProject',
    ENTER_BUCKET_PASSPHRASE = 'enterBucketPassphrase',
    MFA_RECOVERY = 'mfaRecovery',
    ENABLE_MFA = 'enableMFA',
    DISABLE_MFA = 'disableMFA',
    ADD_TOKEN_FUNDS = 'addTokenFunds',
    SHARE = 'share',
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
    DELETE_ACCESS_GRANT = 'deleteAccessGrant',
    SKIP_PASSPHRASE = 'skipPassphrase',
    CHANGE_PROJECT_LIMIT = 'changeProjectLimit',
    REQUEST_PROJECT_LIMIT_INCREASE = 'requestProjectLimitIncrease',
    JOIN_PROJECT = 'joinProject',
    DETAILED_USAGE_REPORT = 'detailedUsageReport',
}

export const MODALS: Record<Modals, Component> = {
    [Modals.ADD_TEAM_MEMBER]: AddTeamMemberModal,
    [Modals.REMOVE_TEAM_MEMBER]: RemoveTeamMemberModal,
    [Modals.EDIT_PROFILE]: EditProfileModal,
    [Modals.CHANGE_PASSWORD]: ChangePasswordModal,
    [Modals.CREATE_PROJECT]: CreateProjectModal,
    [Modals.ENTER_BUCKET_PASSPHRASE]: EnterBucketPassphraseModal,
    [Modals.MFA_RECOVERY]: MFARecoveryCodesModal,
    [Modals.ENABLE_MFA]: EnableMFAModal,
    [Modals.DISABLE_MFA]: DisableMFAModal,
    [Modals.ADD_TOKEN_FUNDS]: AddTokenFundsModal,
    [Modals.SHARE]: ShareModal,
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
    [Modals.DELETE_ACCESS_GRANT]: DeleteAccessGrantModal,
    [Modals.SKIP_PASSPHRASE]: SkipPassphraseModal,
    [Modals.CHANGE_PROJECT_LIMIT]: ChangeProjectLimitModal,
    [Modals.REQUEST_PROJECT_LIMIT_INCREASE]: RequestProjectLimitModal,
    [Modals.JOIN_PROJECT]: JoinProjectModal,
    [Modals.DETAILED_USAGE_REPORT]: DetailedUsageReportModal,
};

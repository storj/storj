// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

export enum ChangeEmailStep {
    InitStep = 0,
    VerifyPasswordStep,
    Verify2faStep,
    VerifyOldEmailStep,
    SetNewEmailStep,
    VerifyNewEmailStep,
    SuccessStep,
}

export enum DeleteAccountStep {
    InitStep = 0,
    VerifyPasswordStep,
    Verify2faStep,
    VerifyEmailStep,
    ConfirmDeleteStep,
    FinalConfirmDeleteStep,
    DeleteBucketsStep,
    DeleteAccessKeysStep,
    PayInvoicesStep,
    WaitForInvoicingStep,
}

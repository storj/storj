// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

export type JoinCunoFSBetaForm = {
    companyName: string;
    industryUseCase: string;
    otherIndustryUseCase: string;
    operatingSystem: string;
    teamSize: string;
    currentStorageUsage: string;
    infraType: string;
    currentStorageBackends: string;
    otherStorageBackend: string;
    currentStorageMountSolution: string;
    otherStorageMountSolution: string;
    desiredFeatures: string;
    currentPainPoints: string;
    specificTasks: string;
};

export type ObjectMountConsultationForm = {
    companyName: string;
    firstName: string;
    lastName: string;
    jobTitle: string;
    phoneNumber: string;
    industryUseCase: string;
    companySize: string;
    currentStorageSolution: string;
    keyChallenges: string;
    specificInterests: string;
    storageNeeds: string;
    implementationTimeline: string;
    additionalInformation: string;
};

export type UserFeedbackForm = {
    type: string;
    message: string;
    allowContact: boolean;
};

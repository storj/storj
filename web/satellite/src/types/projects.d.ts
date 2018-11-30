// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

// Project is a type, used for creating new project in backend
declare type Project = {
    id: string,
    ownerName: string,

    name: string,
    description: string
    isTermsAccepted: boolean,
    createdAt: string

    isSelected: boolean,
}

declare type UpdateProjectModel = {
    id: string,
    description: string
}

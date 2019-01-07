// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

declare type User = {
	firstName: string,
	lastName: string,
	email: string
}

// Used in users module to pass parameters to action
declare type UpdatePasswordModel = {
    oldPassword: string,
	newPassword: string
}

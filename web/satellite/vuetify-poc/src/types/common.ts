// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export type ValidationRule<T> = string | boolean | ((value: T) => string | boolean);
export function RequiredRule(value: unknown): string | boolean {
    return (Array.isArray(value) ? !!value.length : !!value) || 'Required';
}

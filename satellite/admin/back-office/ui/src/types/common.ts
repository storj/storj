// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export type ValidationRule<T> = string | boolean | ((value: T) => string | boolean);
export function RequiredRule(value: unknown): string | boolean {
    return (Array.isArray(value) ? !!value.length : !!value || typeof value === 'number') || 'Required';
}

// TODO: fully implement these types and their methods according to their Go counterparts
export type UUID = string;
export type MemorySize = string;
export type Time = string;

export type DataTableHeader = {
    key: string;
    title: string;
    align?: 'start' | 'end' | 'center';
    sortable?: boolean;
    width?: number | string;
};

export type SortItem = {
    key: string;
    order?: boolean | 'asc' | 'desc';
};
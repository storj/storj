// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { Validator } from '@/utils/validation';

export type ValidationRule<T> = string | boolean | ((value: T) => string | boolean);
export function RequiredRule(value: unknown): string | boolean {
    return (Array.isArray(value) ? !!value.length : !!value) || 'Required';
}

export function EmailRule(value: string): string | boolean {
    return Validator.email(value) || 'E-mail must be valid.';
}

export interface DialogStepComponent {
    title: string;
    iconSrc?: string;
    onEnter?: () => void;
    onExit?: (to: 'next' | 'prev') => void;
    validate?: () => boolean;
}

export type SaveButtonsItem = string | {
    name: string;
    value: string;
};

export const MAX_SEARCH_VALUE_LENGTH = 200;

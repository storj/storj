// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export type ValidationRule<T> = string | boolean | ((value: T) => string | boolean);
export function RequiredRule(value: unknown): string | boolean {
    return (Array.isArray(value) ? !!value.length : !!value) || 'Required';
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

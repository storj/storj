// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { BytesMustBeWholeRule, PositiveNumberRule, RequiredRule } from '@/types/common';
import { Memory } from '@/utils/bytesSize';

export interface FieldRule {
    (value: unknown): boolean | string;
}

export enum FieldType {
    Text,
    Number,
    Select,
    Date,
}

export interface FormField {
    key: string;
    type: FieldType;
    label: string;
    placeholder?: string;
    rules?: FieldRule[];
    disabled?: boolean;
    readonly?: boolean;
    required?: boolean;
    clearable?: boolean;
    cols?: { default?: number; sm?: number; };

    // Select specific
    items?: unknown[];
    itemTitle?: string;
    itemValue?: string;

    // Number specific
    step?: number;
    precision?: number;
    min?: number | Date;
    max?: number | Date;

    prependIcon?: string;

    // Messages and validation
    messages?: ((value: unknown) => string[]);
    errorMessages?: ((value: unknown) => string | undefined);

    // Custom transform functions
    transform?: {
        forward?: (value: unknown) => unknown;
        back?: (value: unknown) => unknown;
    };

    // Visibility condition
    visible?: (formData: unknown) => boolean;

    // Custom update handler
    onUpdate?: (value: unknown) => void;
}

export interface FormRow {
    fields: FormField[];
}

export interface FormSection {
    rows: FormRow[];
    divider?: {
        text?: string;
    };
}

export interface FormConfig {
    sections: FormSection[];
}

export interface FormBuilderExpose {
    getData: () => Record<string, unknown>;
    reset: () => void;
}

export function terabyteFormField(conf: Partial<FormField>): FormField {
    return {
        type: FieldType.Number,
        rules: [RequiredRule, PositiveNumberRule, BytesMustBeWholeRule],
        label: conf.label ?? '',
        key: conf.key ?? '',
        clearable: conf.clearable,
        cols: conf.cols,
        precision: 4,
        step: 0.5,
        messages: (value) => [`Bytes: ${value || 0}`],
        transform: {
            forward: (value) => Number(value) / Memory.TB,
            back: (value) => Number(value) * Memory.TB,
        },
    };
}

export function rawNumberField(conf: Partial<FormField>): FormField {
    return {
        type: FieldType.Number,
        rules: [RequiredRule, PositiveNumberRule],
        key: conf.key ?? '',
        label: conf.label ?? '',
        step: conf.step,
        cols: conf.cols,
    };
}
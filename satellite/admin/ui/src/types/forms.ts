// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { BytesMustBeWholeRule, PositiveNumberRule, RequiredRule } from '@/types/common';
import { Memory } from '@/utils/bytesSize';

/**
 * Special value to represent null for nullable number fields.
 * This value will be saved as null in the DB.
 */
export const NULLABLE_FIELD_VALUE = -1;

export interface FieldRule {
    (value: unknown): boolean | string;
}

export enum FieldType {
    Text,
    TextArea,
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
    const rules = [RequiredRule, PositiveNumberRule, BytesMustBeWholeRule];
    if (conf.clearable) {
        rules.splice(0, 1); // Remove RequiredRule if field is clearable
    }
    return {
        type: FieldType.Number,
        rules,
        label: conf.label ?? '',
        key: conf.key ?? '',
        clearable: conf.clearable,
        cols: conf.cols,
        precision: 4,
        step: 0.5,
        messages: (value) => {
            const bytes = [`Bytes: ${value || 0}`];
            if (!conf.clearable) return bytes;

            if (value === null || value === undefined || value === NULLABLE_FIELD_VALUE) {
                return [];
            }
            return bytes;
        },
        transform: {
            forward: (value) => {
                if (!conf.clearable) return Number(value) / Memory.TB;
                return value === NULLABLE_FIELD_VALUE ? null : Number(value) / Memory.TB;
            },
            back: (value) => {
                if (!conf.clearable) return Number(value) * Memory.TB;
                return value === null || value === undefined ? NULLABLE_FIELD_VALUE : Number(value) * Memory.TB;
            },
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

export function nullableNumberField(conf: Partial<FormField>): FormField {
    return {
        type: FieldType.Number,
        rules: [PositiveNumberRule],
        key: conf.key ?? '',
        label: conf.label ?? '',
        clearable: true,
        step: conf.step ?? 100,
        cols: conf.cols ?? { default: 12, sm: 4 },
        transform: {
            forward: (value) => value === NULLABLE_FIELD_VALUE ? null : value,
            back: (value) => value === null || value === undefined ? NULLABLE_FIELD_VALUE : value,
        },
    };
}
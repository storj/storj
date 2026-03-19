// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

export const licenseTypeOptions = [
    { label: 'Object-Mount', value: 'OM' },
];

/**
 * Returns the display label for a license type value.
 * Falls back to the raw value if not found.
 */
export function licenseTypeLabel(value: string): string {
    return licenseTypeOptions.find(o => o.value === value)?.label ?? value;
}

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Represents a generic JSON object.
 */
export type JSONObject = string | number | boolean | null | JSONObject[] | {
    [key: string]: [value: JSONObject];
};

/**
 * Represents a JSON object which is a subset of T containing only properties
 * whose values are JSON-representable.
 */
export type JSONRepresentable<T> =
    T extends undefined ? never :
        T extends JSONObject ? T :
            Pick<T, {
                [P in keyof T]: T[P] extends JSONObject ? P : never;
            }[keyof T]>;

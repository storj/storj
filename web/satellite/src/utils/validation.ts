// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Validator holds validation check methods for strings.
 */
export class Validator {
    /**
     * Checks string to satisfy email rules.
     * @param email - email to check.
     * @param strict - if true, checks for stricter email rules.
     */
    public static email(email: string, strict = false): boolean {
        let rgx = /.*@.*\..*$/;

        if (strict) {
            // We'll have this email validation for new users instead of using regular Validator.email method because of backwards compatibility.
            // We don't want to block old users who managed to create and verify their accounts with some weird email addresses.

            // This regular expression fulfills our needs to validate international emails.
            // It was built according to RFC 5322 and then extended to include international characters using these resources
            // https://emailregex.com/
            // https://awik.io/international-email-address-validation-javascript/
            // eslint-disable-next-line no-misleading-character-class
            rgx = /^(([^<>()[\]\\.,;:\s@"]+(\.[^<>()[\]\\.,;:\s@"]+)*)|(".+"))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}])|(([a-zA-Z\-0-9\u0080-\u00FF\u0100-\u017F\u0180-\u024F\u0250-\u02AF\u0300-\u036F\u0370-\u03FF\u0400-\u04FF\u0500-\u052F\u0530-\u058F\u0590-\u05FF\u0600-\u06FF\u0700-\u074F\u0750-\u077F\u0780-\u07BF\u07C0-\u07FF\u0900-\u097F\u0980-\u09FF\u0A00-\u0A7F\u0A80-\u0AFF\u0B00-\u0B7F\u0B80-\u0BFF\u0C00-\u0C7F\u0C80-\u0CFF\u0D00-\u0D7F\u0D80-\u0DFF\u0E00-\u0E7F\u0E80-\u0EFF\u0F00-\u0FFF\u1000-\u109F\u10A0-\u10FF\u1100-\u11FF\u1200-\u137F\u1380-\u139F\u13A0-\u13FF\u1400-\u167F\u1680-\u169F\u16A0-\u16FF\u1700-\u171F\u1720-\u173F\u1740-\u175F\u1760-\u177F\u1780-\u17FF\u1800-\u18AF\u1900-\u194F\u1950-\u197F\u1980-\u19DF\u19E0-\u19FF\u1A00-\u1A1F\u1B00-\u1B7F\u1D00-\u1D7F\u1D80-\u1DBF\u1DC0-\u1DFF\u1E00-\u1EFF\u1F00-\u1FFF\u20D0-\u20FF\u2100-\u214F\u2C00-\u2C5F\u2C60-\u2C7F\u2C80-\u2CFF\u2D00-\u2D2F\u2D30-\u2D7F\u2D80-\u2DDF\u2F00-\u2FDF\u2FF0-\u2FFF\u3040-\u309F\u30A0-\u30FF\u3100-\u312F\u3130-\u318F\u3190-\u319F\u31C0-\u31EF\u31F0-\u31FF\u3200-\u32FF\u3300-\u33FF\u3400-\u4DBF\u4DC0-\u4DFF\u4E00-\u9FFF\uA000-\uA48F\uA490-\uA4CF\uA700-\uA71F\uA800-\uA82F\uA840-\uA87F\uAC00-\uD7AF\uF900-\uFAFF]+\.)+[a-zA-Z\u0080-\u00FF\u0100-\u017F\u0180-\u024F\u0250-\u02AF\u0300-\u036F\u0370-\u03FF\u0400-\u04FF\u0500-\u052F\u0530-\u058F\u0590-\u05FF\u0600-\u06FF\u0700-\u074F\u0750-\u077F\u0780-\u07BF\u07C0-\u07FF\u0900-\u097F\u0980-\u09FF\u0A00-\u0A7F\u0A80-\u0AFF\u0B00-\u0B7F\u0B80-\u0BFF\u0C00-\u0C7F\u0C80-\u0CFF\u0D00-\u0D7F\u0D80-\u0DFF\u0E00-\u0E7F\u0E80-\u0EFF\u0F00-\u0FFF\u1000-\u109F\u10A0-\u10FF\u1100-\u11FF\u1200-\u137F\u1380-\u139F\u13A0-\u13FF\u1400-\u167F\u1680-\u169F\u16A0-\u16FF\u1700-\u171F\u1720-\u173F\u1740-\u175F\u1760-\u177F\u1780-\u17FF\u1800-\u18AF\u1900-\u194F\u1950-\u197F\u1980-\u19DF\u19E0-\u19FF\u1A00-\u1A1F\u1B00-\u1B7F\u1D00-\u1D7F\u1D80-\u1DBF\u1DC0-\u1DFF\u1E00-\u1EFF\u1F00-\u1FFF\u20D0-\u20FF\u2100-\u214F\u2C00-\u2C5F\u2C60-\u2C7F\u2C80-\u2CFF\u2D00-\u2D2F\u2D30-\u2D7F\u2D80-\u2DDF\u2F00-\u2FDF\u2FF0-\u2FFF\u3040-\u309F\u30A0-\u30FF\u3100-\u312F\u3130-\u318F\u3190-\u319F\u31C0-\u31EF\u31F0-\u31FF\u3200-\u32FF\u3300-\u33FF\u3400-\u4DBF\u4DC0-\u4DFF\u4E00-\u9FFF\uA000-\uA48F\uA490-\uA4CF\uA700-\uA71F\uA800-\uA82F\uA840-\uA87F\uAC00-\uD7AF\uF900-\uFAFF]{2,}))$/;
        }

        return rgx.test(email);
    }

    /**
     * Checks string to satisfy bucket name rules.
     */
    public static bucketName(value: string): boolean {
        const rgx = /^[a-z0-9][a-z0-9.-]+[a-z0-9]$/;

        return rgx.test(value);
    }

    /**
     * Checks string to satisfy domain name rules.
     */
    public static domainName(value: string): boolean {
        const rgx = /^(?:[a-zA-Z0-9-]+\.)+[a-zA-Z]{2,63}$/;

        return rgx.test(value);
    }

    /**
     * Checks string to satisfy hostname rules.
     */
    public static hostname(hostname: string): boolean {
        if (hostname.length > 255) return false;

        // require at least one dot and each label 1..63, no leading/trailing hyphen.
        const rgx = /^(?=.{1,255}$)(?!-)[A-Za-z0-9-]{1,63}(?<!-)(\.(?!-)[A-Za-z0-9-]{1,63}(?<!-))+$/;
        return rgx.test(hostname);
    }

    /**
     * Checks if value string is less than or equal to max possible length.
     */
    public static nameLength(value: string, maxLength: number): boolean {
        return value.length <= maxLength;
    }

    /**
     * Checks string to see if it contains typical phone number characters.
     */
    public static phoneNumber(value: string): boolean {
        const rgx = /^\+?\d{1,15}(?:[\s.-]?\d{1,15})*$/;
        return rgx.test(value);
    }

    /**
     * Checks string to see if it is a valid SSH public key.
     * Ported from Compute backend (Go) implementation of isPublicKeyValid and readSSHProtoString.
     * Mirrors logic defined in RFC 4251, 4253, 5656, and 8709.
     * Original reference: https://pkg.go.dev/golang.org/x/crypto/ssh and internal backend code at func isPublicKeyValid(key string) bool
     */
    public static publicSSHKey(key: string): boolean {
        if (key.length > 8192) return false;

        const parts = key.trim().split(/\s+/, 3);
        if (parts.length < 2) return false;

        const keyFormat = parts[0];
        const blobStr = parts[1];

        const blobBytes = base64ToBytes(blobStr);
        if (!blobBytes) return false;

        let blobFormat: Uint8Array;
        let rest: Uint8Array;
        let ok: boolean;

        // eslint-disable-next-line prefer-const
        [blobFormat, rest, ok] = readSSHProtoString(blobBytes);
        if (!ok || bytesToString(blobFormat) !== keyFormat) return false;

        switch (keyFormat) {
        // RFC 4253
        case 'ssh-rsa':
            for (let i = 0; i < 2; i++) {
                [, rest, ok] = readSSHProtoString(rest);
                if (!ok) return false;
            }
            break;
        case 'ssh-dss':
            for (let i = 0; i < 4; i++) {
                [, rest, ok] = readSSHProtoString(rest);
                if (!ok) return false;
            }
            break;
        // RFC 8709
        case 'ssh-ed25519': {
            let pubKey: Uint8Array;
            [pubKey, rest, ok] = readSSHProtoString(rest);
            if (!ok || pubKey.length !== 32) return false;
            break;
        }
        // RFC 5656
        case 'ecdsa-sha2-nistp256':
        case 'ecdsa-sha2-nistp384':
        case 'ecdsa-sha2-nistp521': {
            let curveID: Uint8Array;
            [curveID, rest, ok] = readSSHProtoString(rest);

            const expectedCurveID = keyFormat.substring(keyFormat.lastIndexOf('-') + 1);
            if (!ok || bytesToString(curveID) !== expectedCurveID) return false;

            [, rest, ok] = readSSHProtoString(rest);
            if (!ok) return false;
            break;
        }
        default:
            return false;
        }

        return rest.length === 0;
    }
}

/**
 * Decodes base64 into bytes; returns null on failure.
 * @param b64
 */
function base64ToBytes(b64: string): Uint8Array | null {
    try {
        const clean = b64.replace(/\s+/g, '');
        const bin = atob(clean);
        const out = new Uint8Array(bin.length);

        for (let i = 0; i < bin.length; i++) {
            out[i] = bin.charCodeAt(i) & 0xff;
        }

        return out;
    } catch {
        return null;
    }
}

/**
 * Read an SSH "string" per RFC 4251 §5: 4-byte big-endian length + bytes.
 * Returns the byte slice, the unread portion of the slice, and whether the read was successful.
 * If the read was unsuccessful, an empty slice will be returned.
 * @param data
 */
function readSSHProtoString(data: Uint8Array): [str: Uint8Array, rest: Uint8Array, ok: boolean] {
    if (data.length < 4) return [new Uint8Array(0), data, false];

    const length = (data[0] << 24 | data[1] << 16 | data[2] << 8 | data[3]) >>> 0;
    if (length > data.length - 4) return [new Uint8Array(0), data, false];

    const str = data.subarray(4, 4 + length);
    const rest = data.subarray(4 + length);

    return [str, rest, true];
}

/**
 * Decode bytes to string (for ASCII-like protocol fields).
 * @param bytes
 */
function bytesToString(bytes: Uint8Array): string {
    return new TextDecoder('utf-8', { fatal: false }).decode(bytes);
}

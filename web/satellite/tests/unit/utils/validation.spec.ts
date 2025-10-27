// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { describe, it, expect } from 'vitest';

import { Validator } from '@/utils/validation';

describe('validation', (): void => {
    it('email regex works correctly', () => {
        const testString1 = 'test';
        const testString2 = '        ';
        const testString3 = 'test@';
        const testString4 = 'test.test';
        const testString5 = 'test1@23.3';
        const testString6 = '';
        const testString7 = '@teSTt.1123';

        expect(Validator.email(testString1)).toBe(false);
        expect(Validator.email(testString2)).toBe(false);
        expect(Validator.email(testString3)).toBe(false);
        expect(Validator.email(testString4)).toBe(false);
        expect(Validator.email(testString5)).toBe(true);
        expect(Validator.email(testString6)).toBe(false);
        expect(Validator.email(testString7)).toBe(true);
    });

    describe('hostname validation', () => {
        it('accepts valid hostnames with dots', () => {
            expect(Validator.hostname('example.com')).toBe(true);
            expect(Validator.hostname('subdomain.example.com')).toBe(true);
            expect(Validator.hostname('deep.subdomain.example.com')).toBe(true);
            expect(Validator.hostname('test-host.example.org')).toBe(true);
            expect(Validator.hostname('api2.staging.example.co.uk')).toBe(true);
        });

        it('rejects hostnames without dots', () => {
            expect(Validator.hostname('localhost')).toBe(false);
            expect(Validator.hostname('hostname')).toBe(false);
            expect(Validator.hostname('single')).toBe(false);
        });

        it('rejects hostnames with leading or trailing hyphens in labels', () => {
            expect(Validator.hostname('-example.com')).toBe(false);
            expect(Validator.hostname('example-.com')).toBe(false);
            expect(Validator.hostname('test.-example.com')).toBe(false);
            expect(Validator.hostname('test.example-.com')).toBe(false);
        });

        it('rejects hostnames exceeding 255 characters', () => {
            const longLabel = 'a'.repeat(64);
            expect(Validator.hostname(`${longLabel}.com`)).toBe(false);

            const label63 = 'a'.repeat(63);
            expect(Validator.hostname(`${label63}.example.com`)).toBe(true);

            const veryLongHostname = 'a'.repeat(250) + '.com';
            expect(Validator.hostname(veryLongHostname)).toBe(false);
        });

        it('rejects empty strings and invalid formats', () => {
            expect(Validator.hostname('')).toBe(false);
            expect(Validator.hostname('.')).toBe(false);
            expect(Validator.hostname('..')).toBe(false);
            expect(Validator.hostname('example.')).toBe(false);
            expect(Validator.hostname('.example.com')).toBe(false);
        });

        it('rejects hostnames with invalid characters', () => {
            expect(Validator.hostname('test_host.com')).toBe(false);
            expect(Validator.hostname('test host.com')).toBe(false);
            expect(Validator.hostname('test@host.com')).toBe(false);
            expect(Validator.hostname('test#host.com')).toBe(false);
        });
    });

    describe('SSH public key validation', () => {
        describe('ssh-rsa keys', () => {
            it('accepts valid ssh-rsa key', () => {
                const validRsaKey = 'ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDXpvih9kNJkXRljyjm6iXCa8/4/ly21KO/uNJbQCIGyKNMPy66CLugMF/W3ZjLK93nIqR2dKWNzottfVwSINMbKkCGh5hFvpluTIykoxK+Wn07koHmNrfoohYAZ4BLKbdNM178liSYuYE/3oJ+da5zN1buM8/u9O7yeH3HvVYoCTz/gASCh85G33xp1AmKfPmmbUJnS+7+pVO6VunsFQDtZXXBIs9HP0wRD57H34BYAK5xs/2bPUD+FC/X+O0MmvsDxsCUgURoQCTjrGzhLZqfySfPlJXcHhdUPXD3OsS1vLwuTYWHbVkEYbdPSudDAO1xOpq0iQKCytmc5wdHvTEX test@example.com';
                expect(Validator.publicSSHKey(validRsaKey)).toBe(true);
            });

            it('accepts ssh-rsa key without comment', () => {
                const keyWithoutComment = 'ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDXpvih9kNJkXRljyjm6iXCa8/4/ly21KO/uNJbQCIGyKNMPy66CLugMF/W3ZjLK93nIqR2dKWNzottfVwSINMbKkCGh5hFvpluTIykoxK+Wn07koHmNrfoohYAZ4BLKbdNM178liSYuYE/3oJ+da5zN1buM8/u9O7yeH3HvVYoCTz/gASCh85G33xp1AmKfPmmbUJnS+7+pVO6VunsFQDtZXXBIs9HP0wRD57H34BYAK5xs/2bPUD+FC/X+O0MmvsDxsCUgURoQCTjrGzhLZqfySfPlJXcHhdUPXD3OsS1vLwuTYWHbVkEYbdPSudDAO1xOpq0iQKCytmc5wdHvTEX';
                expect(Validator.publicSSHKey(keyWithoutComment)).toBe(true);
            });

            it('rejects ssh-rsa key with invalid base64', () => {
                const invalidBase64Key = 'ssh-rsa INVALID-BASE64-DATA!@#$ user@example.com';
                expect(Validator.publicSSHKey(invalidBase64Key)).toBe(false);
            });

            it('rejects ssh-rsa key with mismatched key type in blob', () => {
                const mismatchedKey = 'ssh-rsa AAAAC3NzaC1lZDI1NTE5AAAAIEt+A9BDG/wxyIB9R9pFy0WUETLMIpSbehROBdESB3bn user@example.com';
                expect(Validator.publicSSHKey(mismatchedKey)).toBe(false);
            });
        });

        describe('ssh-ed25519 keys', () => {
            it('accepts valid ssh-ed25519 key', () => {
                const validEd25519Key = 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEt+A9BDG/wxyIB9R9pFy0WUETLMIpSbehROBdESB3bn test@example.com';
                expect(Validator.publicSSHKey(validEd25519Key)).toBe(true);
            });

            it('rejects ssh-ed25519 key with incorrect public key length', () => {
                const invalidLengthKey = 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIDExMjM0NTY3ODkwMQ== user@example.com';
                expect(Validator.publicSSHKey(invalidLengthKey)).toBe(false);
            });
        });

        describe('ssh-dss keys', () => {
            it('accepts valid ssh-dss key', () => {
                const validDssKey = 'ssh-dss AAAAB3NzaC1kc3MAAACBAMtc6ZrNR2rIJxE1j3G6KBtL1MSxLpIcv3M5jIiO602ik52BG+X3An3rKLdm4tYjXzW8Ays6q5ikRRSp2z+qtpq8l0ewI1L/COjq1W+Wh3KoRXwkzDKZpn5dRIOrC8YwsR9/V8pjjYQeryOceXiDsboYpjwWzplE74LPll2FoUZbAAAAFQDwcSJpASpxgEWVN8/JT6ht40b7jwAAAIBEqzi2r+E52XyMdo2G4gn4FvdK9op+CuWhTTfoZiWdWjaGhMymZPYL9XKKoFEg65UcscgPPQdoiulsoZs3PGO8nLzqSlZ3JLDDtsVGYnMy6QK+ZkK5MD+BxUhaWLk/gKXJIBg89cLiVXv4KSCY6dBgGBDkQ291nsA+eUDLd8+CuAAAAIB8kYOBXK26uLzEHBTZd1NhuE3RlVfa44XAo5uPX7CM1Z73xouJVgTx7AtvIxaJDE+uzSEzgBItsRUJNWKWoGzvH5HsU+IoukOdh6c+SZh6vj8NZFZB7U8B6vAwDkJHsqfv898B+ZpEt3fz9WrvN4ZNysrDN4jYLGPRNUc3F8Xj9Q== test@example.com';
                expect(Validator.publicSSHKey(validDssKey)).toBe(true);
            });
        });

        describe('ecdsa keys', () => {
            it('accepts valid ecdsa-sha2-nistp256 key', () => {
                const validEcdsaKey = 'ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBPTCpWkxP+ZSw4157PgXTB4OxOnLlm/3jtPpudHqmNo5gCZGgLwbi6N2xexEAygH/LFsaPvG9iK0T9WMrWtl/X4= test@example.com';
                expect(Validator.publicSSHKey(validEcdsaKey)).toBe(true);
            });

            it('accepts valid ecdsa-sha2-nistp384 key', () => {
                const validEcdsaKey = 'ecdsa-sha2-nistp384 AAAAE2VjZHNhLXNoYTItbmlzdHAzODQAAAAIbmlzdHAzODQAAABhBDPA3Z1pOohluoxz77NptEstfouqOISe5UdDQn/g50cbzS0IlrLNOal01jRdkoTH6SU0D6rZtNaM1OqfKcpnV1MWpGGMVLY2nX0XdQvtJD4ZQlwUxBhiHUBUVdxAgaMcMQ== test@example.com';
                expect(Validator.publicSSHKey(validEcdsaKey)).toBe(true);
            });

            it('accepts valid ecdsa-sha2-nistp521 key', () => {
                const validEcdsaKey = 'ecdsa-sha2-nistp521 AAAAE2VjZHNhLXNoYTItbmlzdHA1MjEAAAAIbmlzdHA1MjEAAACFBAAf6qhBa3VMNqUB3USZN2jCRQYF/itA61r5zm6Tv+EL/wytOH6UHYXhSZSU2gSZKEG0gF4alx+hOgICEMtL1oBD0wF+vlLip7X+oOpeslN+yZtQGellX7PRgnH20M+RolmWkDWzgQ9flX6LDy3JmzUoPF09qbcdiPkuCkZX/TRvNGNvKw== test@example.com';
                expect(Validator.publicSSHKey(validEcdsaKey)).toBe(true);
            });

            it('rejects ecdsa key with mismatched curve ID', () => {
                const mismatchedCurveKey = 'ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAzODQAAABhBDPA3Z1pOohluoxz77NptEstfouqOISe5UdDQn/g50cbzS0IlrLNOal01jRdkoTH6SU0D6rZtNaM1OqfKcpnV1MWpGGMVLY2nX0XdQvtJD4ZQlwUxBhiHUBUVdxAgaMcMQ== user@example.com';
                expect(Validator.publicSSHKey(mismatchedCurveKey)).toBe(false);
            });
        });

        describe('general validation rules', () => {
            it('rejects keys exceeding 8192 characters', () => {
                const tooLongKey = 'ssh-rsa ' + 'A'.repeat(8200);
                expect(Validator.publicSSHKey(tooLongKey)).toBe(false);
            });

            it('rejects empty string', () => {
                expect(Validator.publicSSHKey('')).toBe(false);
            });

            it('rejects key with only key type (missing blob)', () => {
                expect(Validator.publicSSHKey('ssh-rsa')).toBe(false);
            });

            it('rejects key with unsupported key type', () => {
                const unsupportedKey = 'ssh-unsupported AAAAB3NzaC1yc2EAAAADAQABAAABgQC7Z9r0T8z0Q9T3k6I8e5P3B3N2xY user@example.com';
                expect(Validator.publicSSHKey(unsupportedKey)).toBe(false);
            });

            it('rejects key with extra data after blob', () => {
                const extraDataKey = 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGPkYbp0d7w3p3b9p3b9p3b9p3b9p3b9p3b9p3b9p3b9EXTRA';
                expect(Validator.publicSSHKey(extraDataKey)).toBe(false);
            });

            it('accepts keys with whitespace and comments', () => {
                const keyWithWhitespace = '  ssh-ed25519   AAAAC3NzaC1lZDI1NTE5AAAAIGPkYbp0d7w3p3b9p3b9p3b9p3b9p3b9p3b9p3b9p3b9   user@example.com  ';
                expect(Validator.publicSSHKey(keyWithWhitespace)).toBe(true);
            });

            it('rejects malformed blob (invalid SSH protocol string)', () => {
                const malformedBlobKey = 'ssh-rsa AAAA user@example.com';
                expect(Validator.publicSSHKey(malformedBlobKey)).toBe(false);
            });
        });
    });
});

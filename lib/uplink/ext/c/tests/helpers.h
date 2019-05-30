// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

EncryptionAccess_t * NewEncryptionAccess(uint8_t *key, int key_len);
void freeEncryptionAccess(EncryptionAccess_t *access);

ProjectRef_t OpenTestProject(char **err);

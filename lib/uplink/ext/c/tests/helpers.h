// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

EncryptionAccess_t * NewEncryptionAccess(uint8_t *key, int key_len);

void FreeEncryptionAccess(EncryptionAccess_t *);
void FreeBucket(Bucket_t *);

ProjectRef_t OpenTestProject(char **err);

Bucket_t *CreateTestBucket(ProjectRef_t, char *bucket_name, char **err);

Bytes_t *BytesFromString(char *str_data);

UplinkRef_t NewTestUplink(char **);

void create_test_object(BucketRef_t ref_bucket, char *path, Object_t *object, Bytes_t *data, char **err);
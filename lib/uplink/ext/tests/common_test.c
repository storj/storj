// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdio.h>
#include <unistd.h>
#include "unity.h"
#include "../uplink-cgo.h"

void TestGetIDVersion(void)
{
    char *_err = "";
    char **err = &_err;
    uint8_t idVersionNumber = 0;

    Value idVersionValue = GetIDVersion(idVersionNumber, err);
    TEST_ASSERT_EQUAL_STRING("", *err);

    Unpack(&idVersionValue, err);
    struct IDVersionProto idVersion
//    IDVersionNumber versionNumber = GetIDVersionNumber(idVersion);
//    TEST_ASSERT_EQUAL(0, versionNumber);
//    TEST_ASSERT_EQUAL(0, idVersionValue.Snapshot.Number);
}

void TestAPIKey(void)
{
    char *_err = "";
    char **err = &_err;
    char *keyStr = "HiBryanIDidIt";
    APIKey apikey = ParseAPIKey(keyStr, err);
    char *resultKey = Serialize(apikey);

    TEST_ASSERT_EQUAL_STRING("", *err);
    TEST_ASSERT_EQUAL_STRING(keyStr, resultKey);
}

void TestValue(void)
{
//    void *ptr = CMalloc(sizeof(uint8_t))

//    uint8_t *ptr = CMalloc(sizeof(uint8_t));
//    *ptr = 56;
//    printf("*ptr %d\n", *ptr);
//    GoPrint(ptr);

//    uint8_t *number = 67;
//////    make it a void pointer and track the size
////    struct Value val = NewValueC(number);
////    printf("val.GoPtr %p\n", val.GoPtr);
//    struct Value value = NewValueC((void*)number, sizeof(uint8_t));
//    value = SnapshotValue(value);
//    printf("value snapshot (%%p) %p\n", value.Snapshot);
//    uint8_t *uintSnapshot = (uint8_t*)value.Snapshot;
//    printf("uintSnapshot (%%p) %p\n", uintSnapshot);
//    printf("uintSnapshot (%%d) %d\n", uintSnapshot);
//    printf("*uintSnapshot (%%d) %d\n", *uintSnapshot);
//    TEST_ASSERT_TRUE(false);
}

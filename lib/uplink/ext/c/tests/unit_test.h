// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <assert.h>
#include <string.h>

#define TEST_ASSERT_EQUAL_STRING(expected, actual)  assert(strcmp(expected, actual) == 0)
#define TEST_ASSERT_NOT_EQUAL(expected, actual)     assert(expected != actual)
#define TEST_ASSERT_EQUAL(expected, actual)         assert(expected == actual)
#define TEST_ASSERT_NOT_NULL(value)                 assert(value != NULL)
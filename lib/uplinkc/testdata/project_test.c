// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <string.h>
#include <stdlib.h>

#include "require.h"
#include "uplink.h"
#include "helpers2.h"

void handle_project(ProjectRef_t project)
{};

int main(int argc, char *argv[]) {
    with_test_project(&handle_project);
}

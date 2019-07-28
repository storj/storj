// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <string.h>
#include <stdlib.h>

#include "require.h"
#include "uplink.h"
#include "helpers.h"

void handle_project(ProjectRef project);

int main(int argc, char *argv[]) {
    with_test_project(&handle_project);
}

void handle_project(ProjectRef project) {
    char *_err = "";
    char **err = &_err;

    char *bucket_names[] = {"test-bucket1", "test-bucket2", "test-bucket3", "test-bucket4"};
    int num_of_buckets = sizeof(bucket_names) / sizeof(bucket_names[0]);

    // TODO: test with different bucket configs
    { // Create buckets
        for (int i=0; i < num_of_buckets; i++) {
            char *bucket_name = bucket_names[i];

            BucketConfig config = test_bucket_config();
            BucketInfo info = create_bucket(project, bucket_name, &config, err);
            require_noerror(*err);

            require(strcmp(bucket_name, info.name) == 0);
            require(info.created != 0);

            require(config.encryption_parameters.cipher_suite == info.encryption_parameters.cipher_suite);
            require(config.encryption_parameters.block_size   == info.encryption_parameters.block_size);

            require(config.redundancy_scheme.algorithm        == info.redundancy_scheme.algorithm);
            require(config.redundancy_scheme.share_size       == info.redundancy_scheme.share_size);
            require(config.redundancy_scheme.required_shares  == info.redundancy_scheme.required_shares);
            require(config.redundancy_scheme.repair_shares    == info.redundancy_scheme.repair_shares);
            require(config.redundancy_scheme.optimal_shares   == info.redundancy_scheme.optimal_shares);
            require(config.redundancy_scheme.total_shares     == info.redundancy_scheme.total_shares);

            free_bucket_info(&info);
        }
    }

    // TODO: test list options
    { // listing buckets
        BucketList bucket_list = list_buckets(project, NULL, err);
        require_noerror(*err);
        require(bucket_list.more == 0);
        require(bucket_list.length == num_of_buckets);
        require(bucket_list.items != NULL);

        for(int i = 0; i < bucket_list.length; i++) {
            BucketInfo *info = &bucket_list.items[i];
            require(strcmp(info->name, bucket_names[i]) == 0);
            require(info->created != 0);
        }

        free_bucket_list(&bucket_list);
    }

    { // getting bucket infos
        for(int i = 0; i < num_of_buckets; i++) {
            char *bucket_name = bucket_names[i];
            BucketInfo info = get_bucket_info(project, bucket_name, err);
            require_noerror(*err);
            require(strcmp(info.name, bucket_names[i]) == 0);
            require(info.created != 0);

            free_bucket_info(&info);
        }
    }

    { // encryption context handling
        char *enc_ctx = "12VtN2sbbn9PvaEvNbNUBiSKnRcSUNxBADwDWGsPY7UV85e82tT6u";

        BucketRef bucket = open_bucket(project, bucket_names[0], enc_ctx, err);
        require_noerror(*err);
        requiref(bucket._handle != 0, "got empty bucket\n");

        // TODO: exercise functions that operate on an open bucket to add assertions
        close_bucket(bucket, err);
        require_noerror(*err);
    }

    { // deleting buckets
        for(int i = 0; i < num_of_buckets; i++) {
            delete_bucket(project, bucket_names[i], err);
            require_noerror(*err);
        }
    }
}

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { AccessType } from '@/types/createAccessGrant';

export const ACCESS_TYPE_LINKS: Record<AccessType, string> = {
    [AccessType.AccessGrant]: 'https://docs.storj.io/dcs/concepts/access/access-grants',
    [AccessType.S3]: 'https://docs.storj.io/dcs/api-reference/s3-compatible-gateway',
    [AccessType.APIKey]: 'https://docs.storj.io/dcs/getting-started/quickstart-uplink-cli/generate-access-grants-and-tokens/generate-a-token',
};

export interface AccessGrantEndDate {
    title: string;
    date: Date | null;
}

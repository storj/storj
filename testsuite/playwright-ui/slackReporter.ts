// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { Block, KnownBlock } from '@slack/types';
import { SummaryResults } from 'playwright-slack-report/dist/src';

export default function generateCustomLayoutSimpleMeta(summaryResults: SummaryResults,
): Array<Block | KnownBlock> {
    const meta: { type: string; text: { type: string; text: string; }; }[] = [];
    if (summaryResults.meta) {
        for (let i = 0; i < summaryResults.meta.length; i += 1) {
            const { key, value } = summaryResults.meta[i];
            meta.push({
                type: 'section',
                text: {
                    type: 'mrkdwn',
                    text: `\n*${key}* :\t${value}`,
                    // text: `*${key}*`,
                },
            });
        }
    }
    return [
        {
            type: 'section',
            text: {
                type: 'mrkdwn',
                text:
                    summaryResults.failed === 0
                        ? ':tada: All tests passed!'
                        : `😭${summaryResults.failed} failure(s) out of ${summaryResults.tests.length} tests`,
            },
        },
        // ...meta,
    ];
}

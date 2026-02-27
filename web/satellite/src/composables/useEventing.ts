// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed } from 'vue';
import {
    GetBucketNotificationConfigurationCommand,
    NotificationConfiguration,
    PutBucketNotificationConfigurationCommand,
} from '@aws-sdk/client-s3';

import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { useAccessGrantWorker } from '@/composables/useAccessGrantWorker';
import { BucketNotificationConfig, BucketNotificationConfiguration, EventType } from '@/types/eventing';

export function useEventing() {
    const agStore = useAccessGrantsStore();
    const configStore = useConfigStore();
    const projectsStore = useProjectsStore();
    const bucketsStore = useBucketsStore();

    const { setPermissions, generateAccess } = useAccessGrantWorker();

    /**
     * Returns object browser api key from store.
     */
    const apiKey = computed((): string => {
        return bucketsStore.state.apiKey;
    });

    /**
     * Ensures edge credentials are available for eventing operations.
     */
    async function ensureEdgeCredentials(bucketName: string): Promise<void> {
        const projectID = projectsStore.state.selectedProject.id;
        const now = new Date();

        if (!apiKey.value) {
            const name = `${configStore.state.config.objectBrowserKeyNamePrefix}${now.getTime()}`;
            const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(name, projectID);
            bucketsStore.setApiKey(cleanAPIKey.secret);
        }

        const inOneHour = new Date(now.setHours(now.getHours() + 1));

        const macaroon = await setPermissions({
            isDownload: false,
            isUpload: true,
            isList: false,
            isDelete: false,
            notAfter: inOneHour.toISOString(),
            buckets: JSON.stringify([bucketName]),
            apiKey: apiKey.value,
        });

        const accessGrant = await generateAccess({
            apiKey: macaroon,
            passphrase: '',
        }, projectsStore.state.selectedProject.id);

        const edgeCredentials: EdgeCredentials = await agStore.getEdgeCredentials(accessGrant);
        bucketsStore.setEdgeCredentialsForEventing(edgeCredentials);
    }

    /**
     * Get bucket notification configuration.
     */
    async function getNotificationConfig(bucketName: string): Promise<BucketNotificationConfiguration> {
        await ensureEdgeCredentials(bucketName);

        const client = bucketsStore.state.s3ClientForEventing;
        const command = new GetBucketNotificationConfigurationCommand({ Bucket: bucketName });
        const response = await client.send(command);

        return {
            topicConfigurations: response.TopicConfigurations?.map(config => {
                const filterRules = config.Filter?.Key?.FilterRules || [];
                const prefix = filterRules.find(r => r.Name === 'prefix')?.Value || '';
                const suffix = filterRules.find(r => r.Name === 'suffix')?.Value || '';
                return {
                    topicArn: config.TopicArn || '',
                    events: (config.Events || []) as EventType[],
                    filterPrefix: prefix,
                    filterSuffix: suffix,
                };
            }) || [],
        };
    }

    /**
     * Update bucket notification configuration.
     */
    async function updateNotificationConfig(
        bucketName: string,
        config: BucketNotificationConfig | null,
    ): Promise<void> {
        await ensureEdgeCredentials(bucketName);

        // Convert to AWS SDK format
        const notificationConfig: NotificationConfiguration = {
            TopicConfigurations: config ? [{
                TopicArn: config.topicArn,
                Events: config.events,
                Filter: (config.filterPrefix || config.filterSuffix) ? {
                    Key: {
                        FilterRules: [
                            ...(config.filterPrefix ? [{ Name: 'prefix' as const, Value: config.filterPrefix }] : []),
                            ...(config.filterSuffix ? [{ Name: 'suffix' as const, Value: config.filterSuffix }] : []),
                        ],
                    },
                } : undefined,
            }] : [],
        };

        const client = bucketsStore.state.s3ClientForEventing;
        const command = new PutBucketNotificationConfigurationCommand({
            Bucket: bucketName,
            NotificationConfiguration: notificationConfig,
        });

        await client.send(command);
    }

    return {
        getNotificationConfig,
        updateNotificationConfig,
    };
}

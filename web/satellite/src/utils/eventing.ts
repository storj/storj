// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

export const ARN_PREFIX = 'arn:gcp:pubsub::';

/**
 * Converts from arn:gcp:pubsub::PROJECT_ID:TOPIC_ID to projects/PROJECT_ID/topics/TOPIC_ID
 */
export function convertArnToTopicName(arn: string): string {
    if (!arn.startsWith(ARN_PREFIX)) {
        throw new Error(`Invalid ARN format: must start with '${ARN_PREFIX}'`);
    }

    const resourcePart = arn.slice(ARN_PREFIX.length);

    const parts = resourcePart.split(':');
    if (parts.length !== 2) {
        throw new Error(`Invalid ARN format: expected 'PROJECT_ID:TOPIC_ID' after '${ARN_PREFIX}'`);
    }

    return `projects/${parts[0]}/topics/${parts[1]}`;
}

/**
 * Parses a fully-qualified topic name in the format projects/PROJECT_ID/topics/TOPIC_ID
 */
export function parseTopicName(fullyQualifiedName: string): { projectId: string; topicId: string } {
    const parts = fullyQualifiedName.split('/');

    if (parts.length !== 4) {
        throw new Error(`Invalid fully-qualified topic name format: ${fullyQualifiedName}`);
    }

    // Validate structure ("projects" and "topics" segments)
    if (parts[0] !== 'projects' || parts[2] !== 'topics') {
        throw new Error(`Invalid fully-qualified topic name format: ${fullyQualifiedName}`);
    }

    const projectId = parts[1];
    const topicId = parts[3];

    return { projectId, topicId };
}
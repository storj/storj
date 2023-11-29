// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

/*
 * This is the Admin API client exposing the API operations using the
 * interfaces, types and classes of the `ui-generator.ts` for allowing the
 * `UIGenerator.svelte` component to dynamically render their Web UI interface.
 */

import type { Operation } from '$lib/ui-generator';
import { InputText, Select } from '$lib/ui-generator';

// API must be implemented by any class which expose the access to a specific
// API.
export interface API {
	operations: {
		[key: string]: Operation[];
	};
}

export class Admin {
	readonly operations = {
		APIKeys: [
			{
				name: 'get',
				desc: 'Get information on the specific API key',
				params: [['API key', new InputText('text', true)]],
				func: async (apiKey: string): Promise<Record<string, unknown>> => {
					return this.fetch('GET', `apikeys/${apiKey}`);
				}
			},
			{
				name: 'delete key',
				desc: 'Delete an API key',
				params: [['API key', new InputText('text', true)]],
				func: async (apiKey: string): Promise<null> => {
					return this.fetch('DELETE', `apikeys/${apiKey}`) as Promise<null>;
				}
			}
		],
		bucket: [
			{
				name: 'get',
				desc: 'Get the information of the specified bucket',
				params: [
					['Project ID', new InputText('text', true)],
					['Bucket name', new InputText('text', true)]
				],
				func: async (projectId: string, bucketName: string): Promise<Record<string, unknown>> => {
					return this.fetch('GET', `projects/${projectId}/buckets/${bucketName}`);
				}
			},
			{
				name: 'delete geofencing',
				desc: 'Delete the geofencing configuration of the specified bucket. The bucket MUST be empty',
				params: [
					['Project ID', new InputText('text', true)],
					['Bucket name', new InputText('text', true)]
				],
				func: async (projectId: string, bucketName: string): Promise<null> => {
					return this.fetch(
						'DELETE',
						`projects/${projectId}/buckets/${bucketName}/geofence`
					) as Promise<null>;
				}
			},
			{
				name: 'set geofencing',
				desc: 'Set the geofencing configuration of the specified bucket. The bucket MUST be empty',
				params: [
					['Project ID', new InputText('text', true)],
					['Bucket name', new InputText('text', true)],
					[
						'Region',
						new Select(false, true, [
							{ text: 'European Union', value: 'EU' },
							{ text: 'European Economic Area', value: 'EEA' },
							{ text: 'United States', value: 'US' },
							{ text: 'Germany', value: 'DE' },
							{ text: 'No Russia and/or other sanctioned country', value: 'NR' }
						])
					]
				],
				func: async (projectId: string, bucketName: string, region: string): Promise<null> => {
					const query = this.urlQueryFromObject({ region });
					if (query === '') {
						throw new APIError('region cannot be empty');
					}

					return this.fetch(
						'POST',
						`projects/${projectId}/buckets/${bucketName}/geofence`,
						query
					) as Promise<null>;
				}
			}
		],
		oauth_clients: [
			{
				name: 'create',
				desc: 'Add a new oauth client',
				params: [
					['Client ID', new InputText('text', true)],
					['Client Secret', new InputText('text', true)],
					['Owner ID (userID)', new InputText('text', true)],
					['Redirect URL', new InputText('text', true)],
					['Application Name', new InputText('text', true)],
					['Application Logo URL', new InputText('text', false)]
				],
				func: async (
					id: string,
					secret: string,
					ownerID: string,
					redirectURL: string,
					appName: string,
					appLogoURL: string
				): Promise<null> => {
					return this.fetch('POST', `oauth/clients`, null, {
						id,
						secret,
						userID: ownerID,
						redirectURL,
						appName,
						appLogoURL
					}) as Promise<null>;
				}
			},
			{
				name: 'update',
				desc: 'Update an existing oauth client',
				params: [
					['Client ID', new InputText('text', true)],
					['Redirect URL', new InputText('text', false)],
					['Application Name', new InputText('text', false)],
					['Application Logo URL', new InputText('text', false)]
				],
				func: async (
					id: string,
					redirectURL: string,
					appName: string,
					appLogoURL: string
				): Promise<null> => {
					return this.fetch('PUT', `oauth/clients/${id}`, null, {
						id,
						redirectURL,
						appName,
						appLogoURL
					}) as Promise<null>;
				}
			},
			{
				name: 'delete',
				desc: 'Remove an oauth client',
				params: [['Client ID', new InputText('text', true)]],
				func: async (clientID: string): Promise<null> => {
					return this.fetch('DELETE', `oauth/clients/${clientID}`, null) as Promise<null>;
				}
			}
		],
		project: [
			{
				name: 'create',
				desc: 'Add a new project to a specific user',
				params: [
					['Owner ID (user ID)', new InputText('text', true)],
					['Project Name', new InputText('text', true)]
				],
				func: async (ownerId: string, projectName: string): Promise<Record<string, unknown>> => {
					return this.fetch('POST', 'projects', null, { ownerId, projectName });
				}
			},
			{
				name: 'delete',
				desc: 'Delete a specific project',
				params: [['Project ID', new InputText('text', true)]],
				func: async (projectId: string): Promise<null> => {
					return this.fetch('DELETE', `projects/${projectId}`) as Promise<null>;
				}
			},
			{
				name: 'get',
				desc: 'Get the information of a specific project',
				params: [['Project ID', new InputText('text', true)]],
				func: async (projectId: string): Promise<Record<string, unknown>> => {
					return this.fetch('GET', `projects/${projectId}`);
				}
			},
			{
				name: 'update',
				desc: 'Update the information of a specific project',
				params: [
					['Project ID', new InputText('text', true)],
					['Project Name', new InputText('text', true)],
					['Description', new InputText('text', false)]
				],
				func: async (
					projectId: string,
					projectName: string,
					description: string
				): Promise<null> => {
					return this.fetch('PUT', `projects/${projectId}`, null, {
						projectName,
						description
					}) as Promise<null>;
				}
			},
			{
				name: 'update user agent',
				desc: 'Update projects user agent',
				params: [
					['Project ID', new InputText('text', true)],
					['User Agent', new InputText('text', true)]
				],
				func: async (projectId: string, userAgent: string): Promise<null> => {
					return this.fetch('PATCH', `projects/${projectId}/useragent`, null, {
						userAgent
					}) as Promise<null>;
				}
			},
			{
				name: 'create API key',
				desc: 'Create a new API key for a specific project',
				params: [
					['Project ID', new InputText('text', true)],
					['API key name', new InputText('text', true)]
				],
				func: async (projectId: string, name: string): Promise<Record<string, unknown>> => {
					return this.fetch('POST', `projects/${projectId}/apikeys`, null, {
						name
					});
				}
			},
			{
				name: 'delete API key',
				desc: 'Delete a API key of a specific project',
				params: [
					['Project ID', new InputText('text', true)],
					['API Key name', new InputText('text', true)]
				],
				func: async (projectId: string, apiKeyName: string): Promise<null> => {
					return this.fetch(
						'DELETE',
						`projects/${projectId}/apikeys/${apiKeyName}`
					) as Promise<null>;
				}
			},
			{
				name: 'get API keys',
				desc: 'Get the API keys of a specific project',
				params: [['Project ID', new InputText('text', true)]],
				func: async (projectId: string): Promise<Record<string, unknown>> => {
					return this.fetch('GET', `projects/${projectId}/apikeys`);
				}
			},
			{
				name: 'get project usage',
				desc: 'Get the current usage of a specific project',
				params: [['Project ID', new InputText('text', true)]],
				func: async (projectId: string): Promise<Record<string, unknown>> => {
					return this.fetch('GET', `projects/${projectId}/usage`);
				}
			},
			{
				name: 'get project limits',
				desc: 'Get the current limits of a specific project',
				params: [['Project ID', new InputText('text', true)]],
				func: async (projectId: string): Promise<Record<string, unknown>> => {
					return this.fetch('GET', `projects/${projectId}/limit`);
				}
			},
			{
				name: 'update project limits',
				desc: 'Update the limits of a specific project',
				params: [
					['Project ID', new InputText('text', true)],
					['Storage (in bytes)', new InputText('number', false)],
					['Bandwidth (in bytes)', new InputText('number', false)],
					['Rate (requests per second)', new InputText('number', false)],
					['Buckets (maximum number)', new InputText('number', false)],
					['Burst (max concurrent requests)', new InputText('number', false)],
					['Segments (maximum number)', new InputText('number', false)]
				],
				func: async (
					projectId: string,
					usage: number,
					bandwidth: number,
					rate: number,
					buckets: number,
					burst: number,
					segments: number
				): Promise<null> => {
					const query = this.urlQueryFromObject({
						usage,
						bandwidth,
						rate,
						buckets,
						burst,
						segments
					});

					if (query === '') {
						throw new APIError('nothing to update, at least one limit must be set');
					}

					return this.fetch('PUT', `projects/${projectId}/limit`, query) as Promise<null>;
				}
			}
		],
		user: [
			{
				name: 'create',
				desc: 'Create a new user account',
				params: [
					['email', new InputText('email', true)],
					['full name', new InputText('text', false)],
					['password', new InputText('password', true)]
				],
				func: async (
					email: string,
					fullName: string,
					password: string
				): Promise<Record<string, unknown>> => {
					return this.fetch('POST', 'users', null, {
						email,
						fullName,
						password
					});
				}
			},
			{
				name: 'delete',
				desc: "Delete a user's account",
				params: [['email', new InputText('email', true)]],
				func: async (email: string): Promise<null> => {
					return this.fetch('DELETE', `users/${email}`) as Promise<null>;
				}
			},
			{
				name: 'get',
				desc: "Get the information of a user's account",
				params: [['email', new InputText('email', true)]],
				func: async (email: string): Promise<Record<string, unknown>> => {
					return this.fetch('GET', `users/${email}`);
				}
			},
			{
				name: 'get users pending deletion',
				desc: 'Get the information of a users pending deletion and have no unpaid invoices',
				params: [
					['Limit', new InputText('number', true)],
					['Page', new InputText('number', true)]
				],
				func: async (limit: number, page: number): Promise<Record<string, unknown>> => {
					return this.fetch('GET', `users-pending-deletion?limit=${limit}&page=${page}`);
				}
			},
			{
				name: 'get user limits',
				desc: 'Get the current limits for a user',
				params: [['email', new InputText('email', true)]],
				func: async (email: string): Promise<Record<string, unknown>> => {
					return this.fetch('GET', `users/${email}/limits`);
				}
			},
			{
				name: 'update',
				desc: `Update the information of a user's account.
Blank fields will not be updated.`,
				params: [
					["current user's email", new InputText('email', true)],
					['new email', new InputText('email', false)],
					['full name', new InputText('text', false)],
					['short name', new InputText('text', false)],
					['password hash', new InputText('text', false)],
					['project limit (max number)', new InputText('number', false)],
					['project storage limit (in bytes)', new InputText('number', false)],
					['project bandwidth limit (in bytes)', new InputText('number', false)],
					['project segment limit (max number)', new InputText('number', false)],
					[
						'paid tier',
						new Select(false, false, [
							{ text: '', value: '' },
							{ text: 'true', value: 'true' },
							{ text: 'false', value: 'false' }
						])
					]
				],
				func: async (
					currentEmail: string,
					email?: string,
					fullName?: string,
					shortName?: string,
					passwordHash?: string,
					projectLimit?: number,
					projectStorageLimit?: number,
					projectBandwidthLimit?: number,
					projectSegmentLimit?: number,
					paidTierStr?: boolean
				): Promise<null> => {
					return this.fetch('PUT', `users/${currentEmail}`, null, {
						email,
						fullName,
						shortName,
						passwordHash,
						projectLimit,
						projectStorageLimit,
						projectBandwidthLimit,
						projectSegmentLimit,
						paidTierStr
					}) as Promise<null>;
				}
			},
			{
				name: "update user's project limits",
				desc: `Update limits for all of user's existing and future projects.
				Blank fields will not be updated.`,
				params: [
					["current user's email", new InputText('email', true)],
					[
						'project storage limit (in bytes or notations like 1GB, 2tb)',
						new InputText('text', false)
					],
					[
						'project bandwidth limit (in bytes or notations like 1GB, 2tb)',
						new InputText('text', false)
					],
					['project segment limit (max number)', new InputText('number', false)]
				],
				func: async (
					currentEmail: string,
					storage?: number,
					bandwidth?: number,
					segment?: number
				): Promise<null> => {
					return this.fetch('PUT', `users/${currentEmail}/limits`, null, {
						storage,
						bandwidth,
						segment
					}) as Promise<null>;
				}
			},
			{
				name: 'update user agent',
				desc: `Update user's user agent.`,
				params: [
					["current user's email", new InputText('email', true)],
					['user agent', new InputText('text', true)]
				],
				func: async (currentEmail: string, userAgent: string): Promise<null> => {
					return this.fetch('PATCH', `users/${currentEmail}/useragent`, null, {
						userAgent
					}) as Promise<null>;
				}
			},
			{
				name: 'activate account/disable bot restriction',
				desc: 'disables account bot restriction by activating it. Must be used only for accounts with PendingBotVerification status.',
				params: [['email', new InputText('email', true)]],
				func: async (email: string): Promise<null> => {
					return this.fetch(
						'PATCH',
						`users/${email}/activate-account/disable-bot-restriction`
					) as Promise<null>;
				}
			},
			{
				name: 'disable MFA',
				desc: "Disable user's mulifactor authentication",
				params: [['email', new InputText('email', true)]],
				func: async (email: string): Promise<null> => {
					return this.fetch('DELETE', `users/${email}/mfa`) as Promise<null>;
				}
			},
			{
				name: 'billing freeze user',
				desc: "insert user into account_freeze_events and set user's limits to zero",
				params: [['email', new InputText('email', true)]],
				func: async (email: string): Promise<null> => {
					return this.fetch('PUT', `users/${email}/billing-freeze`) as Promise<null>;
				}
			},
			{
				name: 'billing unfreeze user',
				desc: "remove user from account_freeze_events and reset user's limits to what is stored in account_freeze_events",
				params: [['email', new InputText('email', true)]],
				func: async (email: string): Promise<null> => {
					return this.fetch('DELETE', `users/${email}/billing-freeze`) as Promise<null>;
				}
			},
			{
				name: 'violation freeze user',
				desc: 'freeze a user for ToS violation, set limits to zero and status to pending deletion',
				params: [['email', new InputText('email', true)]],
				func: async (email: string): Promise<null> => {
					return this.fetch('PUT', `users/${email}/violation-freeze`) as Promise<null>;
				}
			},
			{
				name: 'violation unfreeze user',
				desc: "remove a user's violation freeze, reinstating their limits and status (Active)",
				params: [['email', new InputText('email', true)]],
				func: async (email: string): Promise<null> => {
					return this.fetch('DELETE', `users/${email}/violation-freeze`) as Promise<null>;
				}
			},
			{
				name: 'legal freeze user',
				desc: 'freeze a user for legal review, set limits to zero and status to legal hold',
				params: [['email', new InputText('email', true)]],
				func: async (email: string): Promise<null> => {
					return this.fetch('PUT', `users/${email}/legal-freeze`) as Promise<null>;
				}
			},
			{
				name: 'legal unfreeze user',
				desc: "remove a user's legal freeze, reinstating their limits and status (Active)",
				params: [['email', new InputText('email', true)]],
				func: async (email: string): Promise<null> => {
					return this.fetch('DELETE', `users/${email}/legal-freeze`) as Promise<null>;
				}
			},
			{
				name: 'unwarn user',
				desc: "Remove a user's warning status",
				params: [['email', new InputText('email', true)]],
				func: async (email: string): Promise<null> => {
					return this.fetch('DELETE', `users/${email}/billing-warning`) as Promise<null>;
				}
			},
			{
				name: 'set geofencing',
				desc: 'Set account level geofence for a user',
				params: [
					['email', new InputText('email', true)],
					[
						'Region',
						new Select(false, true, [
							{ text: 'European Union', value: 'EU' },
							{ text: 'European Economic Area', value: 'EEA' },
							{ text: 'United States', value: 'US' },
							{ text: 'Germany', value: 'DE' },
							{ text: 'No Russia and/or other sanctioned country', value: 'NR' }
						])
					]
				],
				func: async (email: string, region: string): Promise<null> => {
					return this.fetch('PATCH', `users/${email}/geofence`, null, {
						region
					}) as Promise<null>;
				}
			},
			{
				name: 'delete geofencing',
				desc: 'Delete account level geofence for a user',
				params: [['email', new InputText('email', true)]],
				func: async (email: string): Promise<null> => {
					return this.fetch('DELETE', `users/${email}/geofence`) as Promise<null>;
				}
			}
		],
		rest_api_keys: [
			{
				name: 'create',
				desc: 'Create a REST key',
				params: [
					["user's email", new InputText('text', true)],
					['expiration', new InputText('text', false)]
				],
				func: async (useremail: string, expiration?: string): Promise<Record<string, unknown>> => {
					return this.fetch('POST', `restkeys/${useremail}`, null, {
						expiration
					});
				}
			},
			{
				name: 'revoke',
				desc: 'Revoke a REST key',
				params: [['api key', new InputText('text', true)]],
				func: async (apikey: string): Promise<Record<string, unknown>> => {
					return this.fetch('PUT', `restkeys/${apikey}/revoke`);
				}
			}
		]
	};

	private readonly baseURL: string;

	constructor(baseURL: string, private readonly authToken: string = '') {
		this.baseURL = baseURL.endsWith('/') ? baseURL.substring(0, baseURL.length - 1) : baseURL;
	}

	protected async fetch(
		method: 'DELETE' | 'GET' | 'POST' | 'PUT' | 'PATCH',
		path: string,
		query?: string,
		data?: Record<string, unknown>
	): Promise<Record<string, unknown> | null> {
		const url = this.apiURL(path, query);
		const headers = new window.Headers();

		if (this.authToken) {
			headers.set('Authorization', this.authToken);
		}

		let body: string;
		if (data) {
			headers.set('Content-Type', 'application/json');
			body = JSON.stringify(data);
		}

		const resp = await window.fetch(url, { method, headers, body });
		if (!resp.ok) {
			let body: Record<string, unknown>;
			if (resp.headers.get('Content-Type') === 'application/json') {
				body = await resp.json();
			}

			throw new APIError('server response error', resp.status, body);
		}

		if (resp.headers.get('Content-Type') === 'application/json') {
			return resp.json();
		}

		return null;
	}

	protected apiURL(path: string, query?: string): string {
		path = path.startsWith('/') ? path.substring(1) : path;

		if (!query) {
			query = '';
		} else {
			query = '?' + query;
		}

		return `${this.baseURL}/${path}${query}`;
	}

	protected urlQueryFromObject(values: Record<string, boolean | number | string>): string {
		const queryParts = [];

		for (const name of Object.keys(values)) {
			const val = values[name];
			if (val === undefined) {
				continue;
			}

			queryParts.push(`${name}=${encodeURIComponent(val)}`);
		}

		return queryParts.join('&');
	}
}

class APIError extends Error {
	constructor(
		public readonly msg: string,
		public readonly responseStatusCode?: number,
		public readonly responseBody?: Record<string, unknown> | string
	) {
		super(msg);
	}
}

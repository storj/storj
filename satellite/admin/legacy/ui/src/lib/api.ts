// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

/*
 * This is the Admin API client exposing the API operations using the
 * interfaces, types and classes of the `ui-generator.ts` for allowing the
 * `UIGenerator.svelte` component to dynamically render their Web UI interface.
 */

import { InputText, Select, Operation } from '$lib/ui-generator';

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
					const query = this.urlQueryFromObject({
						name: apiKeyName
					});
					return this.fetch('DELETE', `projects/${projectId}/apikeys`, query) as Promise<null>;
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
				name: 'set placement',
				desc: 'Set placement (NOTE: It will set it for empty and NON-empty projects)',
				params: [
					['Project ID', new InputText('text', true)],
					['Placement ID', new InputText('number', true)]
				],
				func: async (projectId: string, id: string): Promise<null> => {
					const query = this.urlQueryFromObject({ id });
					return this.fetch('PUT', `projects/${projectId}/placement`, query) as Promise<null>;
				}
			},
			{
				name: 'update compute access token',
				desc: `Update project's compute access token.`,
				params: [
					['Project ID', new InputText('text', true)],
					['Access Token (string value or null)', new InputText('text', true)]
				],
				func: async (projectId: string, accessToken: string): Promise<null> => {
					return this.fetch('PATCH', `projects/${projectId}/compute-access-token`, null, {
						accessToken: accessToken.toUpperCase() === 'NULL' ? null : accessToken
					}) as Promise<null>;
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
				name: 'get users pending deletion',
				desc: 'Get the information of a users pending deletion and have no unpaid invoices',
				params: [
					['Limit', new InputText('number', true)],
					['Page', new InputText('number', true)]
				],
				func: async (limit: number, page: number): Promise<Record<string, unknown>> => {
					return this.fetch('GET', `users/deletion/pending?limit=${limit}&page=${page}`);
				}
			},
			{
				name: 'get users requested for deletion',
				desc: 'Get a CSV of user account emails which were requested for deletion by users themselves',
				params: [
					['status updated before (date string i.e. YYYY/MM/DD)', new InputText('text', true)]
				],
				func: async (date: string): Promise<void> => {
					return this.download(
						`users/deletion/requested-by-user?before=${this.toISOStringWithLocalTimezone(date)}`,
						'users-requested-for-deletion.csv'
					);
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
				name: 'activate account/disable bot restriction',
				desc: 'Disables account bot restrictions by activating the account and restoring its limit values. This is used only for accounts with the PendingBotVerification status.',
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
				desc: `insert user into account_freeze_events and set user's limits to zero`,
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
				name: 'trial expiration freeze user',
				desc: 'freeze a user for trial expiration, setting limits to zero',
				params: [['email', new InputText('email', true)]],
				func: async (email: string): Promise<null> => {
					return this.fetch('PUT', `users/${email}/trial-expiration-freeze`) as Promise<null>;
				}
			},
			{
				name: 'trial expiration unfreeze user',
				desc: "remove a user's trial expiration freeze, reinstating their limits",
				params: [['email', new InputText('email', true)]],
				func: async (email: string): Promise<null> => {
					return this.fetch('DELETE', `users/${email}/trial-expiration-freeze`) as Promise<null>;
				}
			},
			{
				name: 'remove billing warning',
				desc: "Remove the billing warning status from a user's account",
				params: [['email', new InputText('email', true)]],
				func: async (email: string): Promise<null> => {
					return this.fetch('DELETE', `users/${email}/billing-warning`) as Promise<null>;
				}
			},
			{
				name: 'update free trial expiration',
				desc: `Update a date when user's free trial will end.`,
				params: [
					["current user's email", new InputText('email', true)],
					[
						'trial expiration date (date string i.e. YYYY/MM/DD or null)',
						new InputText('text', true)
					]
				],
				func: async (currentEmail: string, date: string): Promise<null> => {
					return this.fetch('PATCH', `users/${currentEmail}/trial-expiration`, null, {
						trialExpiration: date === 'null' ? null : this.toISOStringWithLocalTimezone(date)
					}) as Promise<null>;
				}
			},
			{
				name: 'update external ID',
				desc: `Update user's external ID.`,
				params: [
					["user's email", new InputText('email', true)],
					['external ID (string value or null)', new InputText('text', true)]
				],
				func: async (currentEmail: string, externalID: string): Promise<null> => {
					return this.fetch('PATCH', `users/${currentEmail}/external-id`, null, {
						externalID: externalID.toUpperCase() === 'NULL' ? null : externalID
					}) as Promise<null>;
				}
			},
			{
				name: 'update user kind',
				desc: 'Set the kind of user to either 0 (Free User), 1 (Paid User) or 2 (NFR User)',
				params: [
					['email', new InputText('email', true)],
					[
						'kind',
						new Select(false, true, [
							{ text: 'Free User', value: 0 },
							{ text: 'Paid User', value: 1 },
							{ text: 'NFR User', value: 2 },
							{ text: 'Member User', value: 3 }
						])
					]
				],
				func: async (email: string, kind: number): Promise<null> => {
					return this.fetch('PUT', `users/${email}/kind/${kind}`) as Promise<null>;
				}
			},
			{
				name: 'set pending deletion ',
				desc: "Set the user to 'pending deletion' status",
				params: [['email', new InputText('email', true)]],
				func: async (email: string): Promise<null> => {
					return this.fetch('PUT', `users/${email}/status/3`) as Promise<null>;
				}
			}
		],
		rest_api_keys: [
			{
				name: 'create',
				desc: 'Create a REST key. The expiration format must be accepted by https://pkg.go.dev/time#ParseDuration (e.g 20d4h20s) and if it is blank the default applies',
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
		],
		buckets: [
			{
				name: 'update bucket placement',
				desc: 'Updates placement for a bucket metainfo and attribution (positive integer or zero)',
				params: [
					['Project ID', new InputText('text', true)],
					['Bucket name', new InputText('text', true)],
					['Placement ID', new InputText('number', true)]
				],
				func: async (projectId: string, bucketName: string, id: number): Promise<null> => {
					const query = this.urlQueryFromObject({ id });
					return this.fetch(
						'PUT',
						`projects/${projectId}/buckets/${bucketName}/placement`,
						query
					) as Promise<null>;
				}
			}
		],
		value_attributions: [
			{
				name: 'update bucket placement',
				desc: 'Updates placement for a bucket (use integer value or NULL to unset)',
				params: [
					['Project ID', new InputText('text', true)],
					['Bucket name', new InputText('text', true)],
					['Placement', new InputText('text', true)]
				],
				func: async (projectId: string, bucketName: string, placement: number): Promise<null> => {
					const query = this.urlQueryFromObject({
						placement
					});
					return this.fetch(
						'PUT',
						`projects/${projectId}/buckets/${bucketName}/value-attributions`,
						query
					) as Promise<null>;
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

	protected async download(path: string, fileName: string): Promise<void> {
		const url = this.apiURL(path, '');
		const headers = new window.Headers();

		if (this.authToken) {
			headers.set('Authorization', this.authToken);
		}

		const response = await window.fetch(url, { method: 'GET', headers });
		if (!response.ok) {
			let body: Record<string, unknown>;
			if (response.headers.get('Content-Type') === 'application/json') {
				body = await response.json();
			}

			throw new APIError('server response error', response.status, body);
		}

		const blob = await response.blob();

		this.downloadBlob(blob, fileName);
	}

	protected downloadBlob(blob: Blob, fileName: string): void {
		const elem = window.document.createElement('a');
		const link = window.URL.createObjectURL(blob);

		elem.href = link;
		elem.download = fileName;
		elem.click();
		window.URL.revokeObjectURL(link);
	}

	protected toISOStringWithLocalTimezone(dateInput: string): string {
		const date = new Date(dateInput);
		// getTimezoneOffset() returns the difference in minutes between local time and UTC,
		// where local time is ahead of UTC (e.g., UTC+2) it returns a negative value,
		// and where local time is behind UTC (e.g., UTC-5) it returns a positive value.
		const timezoneOffset = date.getTimezoneOffset() * 60000; // convert offset to milliseconds

		// Adjusting the date by its own timezone offset ensures the date part remains unchanged
		// when converting to the ISO string, regardless of the local timezone.
		const adjustedDate = new Date(date.getTime() - timezoneOffset);
		return adjustedDate.toISOString();
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

	protected nullToUndefined<Type>(val: Type): Type {
		if (val === null) {
			return undefined;
		}

		return val;
	}

	protected emptyToUndefined(s: string): string {
		if (s === '') {
			return undefined;
		}

		return s;
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

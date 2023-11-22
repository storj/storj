// AUTOGENERATED BY private/apigen
// DO NOT EDIT.
import { Time, UUID } from '@/types/common';

export class Document {
    id: UUID;
    date: Time;
    pathParam: string;
    body: string;
    version: Version;
    metadata: Metadata;
}

export class Metadata {
    owner?: string;
    tags: string[][] | null;
}

export class NewDocument {
    content: string;
}

export class User {
    name: string;
    surname: string;
    email: string;
}

export class Version {
    date: Time;
    number: number;
}

class APIError extends Error {
    constructor(
        public readonly msg: string,
        public readonly responseStatusCode?: number,
    ) {
        super(msg);
    }
}

export class DocumentsHttpApiV0 {
    public readonly respStatusCode: number;

    // When respStatuscode is passed, the client throws an APIError on each method call
    // with respStatusCode as HTTP status code.
    // respStatuscode must be equal or greater than 400
    constructor(respStatusCode?: number) {
        if (typeof respStatusCode === 'undefined') {
            this.respStatusCode = 0;
            return;
        }

        if (respStatusCode < 400) {
            throw new Error('invalid response status code for API Error, it must be greater or equal than 400');
        }

        this.respStatusCode = respStatusCode;
    }

    public async get(): Promise<Document[]> {
        if (this.respStatusCode !== 0) {
            throw new APIError('mock error message: ' + this.respStatusCode, this.respStatusCode);
        }

        return JSON.parse('[{"id":"00000000-0000-0000-0000-000000000000","date":"0001-01-01T00:00:00Z","pathParam":"/workspace/notes.md","body":"","version":{"date":"0001-01-01T00:00:00Z","number":0},"metadata":{"owner":"Storj","tags":[["category","general"]]}}]') as Document[];
    }

    public async getOne(path: string): Promise<Document> {
        if (this.respStatusCode !== 0) {
            throw new APIError('mock error message: ' + this.respStatusCode, this.respStatusCode);
        }

        return JSON.parse('{"id":"00000000-0000-0000-0000-000000000000","date":"2001-02-02T04:05:06.000000007Z","pathParam":"ID","body":"## Notes","version":{"date":"2001-02-03T03:35:06.000000007Z","number":1},"metadata":{"tags":null}}') as Document;
    }

    public async getTag(path: string, tagName: string): Promise<string[]> {
        if (this.respStatusCode !== 0) {
            throw new APIError('mock error message: ' + this.respStatusCode, this.respStatusCode);
        }

        return JSON.parse('["category","notes"]') as string[];
    }

    public async getVersions(path: string): Promise<Version[]> {
        if (this.respStatusCode !== 0) {
            throw new APIError('mock error message: ' + this.respStatusCode, this.respStatusCode);
        }

        return JSON.parse('[{"date":"2001-01-19T04:05:06.000000007Z","number":1},{"date":"2001-02-02T23:05:06.000000007Z","number":2}]') as Version[];
    }

    public async updateContent(request: NewDocument, path: string, id: UUID, date: Time): Promise<Document> {
        if (this.respStatusCode !== 0) {
            throw new APIError('mock error message: ' + this.respStatusCode, this.respStatusCode);
        }

        return JSON.parse('{"id":"00000000-0000-0000-0000-000000000000","date":"2001-02-03T04:05:06.000000007Z","pathParam":"ID","body":"## Notes\n### General","version":{"date":"0001-01-01T00:00:00Z","number":0},"metadata":{"tags":null}}') as Document;
    }
}

export class UsersHttpApiV0 {
    public readonly respStatusCode: number;

    // When respStatuscode is passed, the client throws an APIError on each method call
    // with respStatusCode as HTTP status code.
    // respStatuscode must be equal or greater than 400
    constructor(respStatusCode?: number) {
        if (typeof respStatusCode === 'undefined') {
            this.respStatusCode = 0;
            return;
        }

        if (respStatusCode < 400) {
            throw new Error('invalid response status code for API Error, it must be greater or equal than 400');
        }

        this.respStatusCode = respStatusCode;
    }

    public async get(): Promise<User[]> {
        if (this.respStatusCode !== 0) {
            throw new APIError('mock error message: ' + this.respStatusCode, this.respStatusCode);
        }

        return JSON.parse('[{"name":"Storj","surname":"Labs","email":"storj@storj.test"},{"name":"Test1","surname":"Testing","email":"test1@example.test"},{"name":"Test2","surname":"Testing","email":"test2@example.test"}]') as User[];
    }

    public async create(request: User[]): Promise<void> {
        if (this.respStatusCode !== 0) {
            throw new APIError('mock error message: ' + this.respStatusCode, this.respStatusCode);
        }

        return;
    }
}

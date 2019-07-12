// APIKeyInfo describing api key model in the database
export class ApiKey {
    public id: string;
    public projectId: string;
    public name: string;
    public createdAt: string;

    constructor()
    constructor(id?: string, projectId?: string, name?: string, createdAt?: string) {
        this.id = id || '';
        this.projectId = projectId || '';
        this.name = name || '';
        this.createdAt = createdAt || '';
    }

    public formattedName(): string {
        let name = this.name;

        if (name.length < 12) {
            return name;
        }

        return name.slice(0, 12) + '...';
    }

    public getDate(): string {
        if (!this.createdAt) {
            return '';
        }

        return new Date(this.createdAt).toLocaleDateString();
    }
}
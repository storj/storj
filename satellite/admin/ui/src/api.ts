import type { Operation } from "./ui-generator";
import { InputText, Select, Textarea } from "./ui-generator";

// API must be implemented by any class which expose the access to a specific
// API.
export interface API {
  operations: {
    [key: string]: Operation[];
  };
}

export class Admin {
  readonly operations = {
    coupons: [
      {
        name: "create",
        desc: "Add a new coupon to a user's account",
        params: [
          ["user ID", new InputText("text", true)],
          ["duration (months)", new InputText("number", true)],
          ["amount (in USD cents)", new InputText("number", true)],
          ["description", new Textarea(true)],
        ],
        func: async (
          userId: string,
          duration: number,
          amount: number,
          description: string
        ): Promise<object> => {
          return this.fetch("POST", "coupons", null, {
            userId,
            duration,
            amount,
            description,
          });
        },
      },
      {
        name: "delete",
        desc: "Delete a specific coupon",
        params: [["Coupon ID", new InputText("text", true)]],
        func: async (couponId: string): Promise<null> => {
          return this.fetch("DELETE", `coupons/${couponId}`) as Promise<null>;
        },
      },
      {
        name: "get",
        desc: "Get the information of a specific coupon",
        params: [["Coupon ID", new InputText("text", true)]],
        func: async (couponId: string): Promise<object> => {
          return this.fetch("GET", `coupons/${couponId}`);
        },
      },
    ],
    project: [
      {
        name: "create",
        desc: "Add a new project to a specific user",
        params: [
          ["Owner ID (user ID)", new InputText("text", true)],
          ["Project Name", new InputText("text", true)],
        ],
        func: async (ownerId: string, projectName: string): Promise<object> => {
          return this.fetch("POST", "projects", null, { ownerId, projectName });
        },
      },
      {
        name: "delete",
        desc: "Delete a specific project",
        params: [["Project ID", new InputText("text", true)]],
        func: async (projectId: string): Promise<null> => {
          return this.fetch("DELETE", `projects/${projectId}`) as Promise<null>;
        },
      },
      {
        name: "get",
        desc: "Get the information of a specific project",
        params: [["Project ID", new InputText("text", true)]],
        func: async (projectId: string): Promise<object> => {
          return this.fetch("GET", `projects/${projectId}`);
        },
      },
      {
        name: "update",
        desc: "Update the information of a specific project",
        params: [
          ["Project ID", new InputText("text", true)],
          ["Project Name", new InputText("text", true)],
          ["Description", new InputText("text", false)],
        ],
        func: async (
          projectId: string,
          projectName: string,
          description: string
        ): Promise<null> => {
          return this.fetch("POST", `projects/${projectId}`, null, {
            projectName,
            description,
          }) as Promise<null>;
        },
      },
      {
        name: "get API keys",
        desc: "Get the API keys of a specific project",
        params: [["Project ID", new InputText("text", true)]],
        func: async (projectId: string): Promise<object> => {
          return this.fetch("GET", `projects/${projectId}/apiKeys`);
        },
      },
      // TODO:continue with the POST /api/projects/{project}/apiKeys endpoint.
    ],
    user: [
      {
        name: "example", // TODO: Delete this endpoint.
        desc: "Example endpoint for reference until all the Admin endpoints are implemented",
        params: [
          ["email", new InputText("email", true)],
          ["full name", new InputText("text", false)],
          ["password", new InputText("password", true)],
          ["Biography", new Textarea(true)],
          [
            "kind",
            new Select(false, true, [
              { text: "", value: "" },
              { text: "personal", value: 1 },
              { text: "business", value: 2 },
            ]),
          ],
        ],
        func: async function (
          email: string,
          fullName: string,
          password: string,
          bio: string,
          accountKind: string
        ): Promise<object> {
          return new Promise((resolve, reject) => {
            window.setTimeout(() => {
              if (Math.round(Math.random()) % 2) {
                resolve({
                  id: "12345678-1234-1234-1234-123456789abc",
                  email: email,
                  fullName: fullName,
                  shortName: fullName.split(" ")[0],
                  bio: bio,
                  passwordLength: password.length,
                  accountKind: accountKind,
                });
              } else {
                reject(
                  new APIError("server response error", 400, {
                    error: "invalid project-uuid",
                    detail: "uuid: invalid string",
                  })
                );
              }
            }, 2000);
          });
        },
      },
      {
        name: "create",
        desc: "Create a new user account",
        params: [
          ["email", new InputText("email", true)],
          ["full name", new InputText("text", false)],
          ["password", new InputText("password", true)],
        ],
        func: async (
          email: string,
          fullName: string,
          password: string
        ): Promise<object> => {
          return this.fetch("POST", "users", null, {
            email,
            fullName,
            password,
          });
        },
      },
      {
        name: "delete",
        desc: "Delete a user's account",
        params: [["email", new InputText("email", true)]],
        func: async (email: string): Promise<null> => {
          return this.fetch("DELETE", `users/${email}`) as Promise<null>;
        },
      },
      {
        name: "get",
        desc: "Get the information of a user's account",
        params: [["email", new InputText("email", true)]],
        func: async (email: string): Promise<object> => {
          return this.fetch("GET", `users/${email}`);
        },
      },
      {
        name: "update",
        desc: `Update the information of a user's account.
Blank fields will not be updated.`,
        params: [
          ["current user's email", new InputText("email", true)],
          ["new email", new InputText("email", false)],
          ["full name", new InputText("text", false)],
          ["short name", new InputText("text", false)],
          ["partner ID", new InputText("text", false)],
          ["password hash", new InputText("text", false)],
        ],
        func: async (
          currentEmail: string,
          email?: string,
          fullName?: string,
          shortName?: string,
          partnerID?: string,
          passwordHash?: string
        ): Promise<null> => {
          return this.fetch("PUT", `users/${currentEmail}`, null, {
            email,
            fullName,
            shortName,
            partnerID,
            passwordHash,
          }) as Promise<null>;
        },
      },
    ],
  };

  private readonly baseURL: string;

  constructor(baseURL: string, private readonly authToken: string) {
    this.baseURL = baseURL.endsWith("/")
      ? baseURL.substring(0, baseURL.length - 1)
      : baseURL;
  }

  protected async fetch(
    method: "DELETE" | "GET" | "POST" | "PUT",
    path: string,
    query?: string,
    data?: object
  ): Promise<object | null> {
    const url = this.apiURL(path, query);
    const headers = new window.Headers({
      Authorization: this.authToken,
    });

    let body: string;
    if (data) {
      headers.set("Content-Type", "application/json");
      body = JSON.stringify(data);
    }

    const resp = await window.fetch(url, { method, headers, body });
    if (!resp.ok) {
      let body: object;
      if (resp.headers.get("Content-Type") === "application/json") {
        body = await resp.json();
      }

      throw new APIError("server response error", resp.status, body);
    }

    if (resp.headers.get("Content-Type") === "application/json") {
      return resp.json();
    }

    return null;
  }

  protected apiURL(path: string, query?: string): string {
    path = path.startsWith("/") ? path.substring(1) : path;

    if (!query) {
      query = "";
    } else {
      query = "?" + query;
    }

    return `${this.baseURL}/${path}${query}`;
  }
}

class APIError extends Error {
  constructor(
    public readonly msg: string,
    public readonly responseStatusCode?: number,
    public readonly responseBody?: object | string
  ) {
    super(msg);
  }
}

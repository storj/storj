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
    ],
  };

  private readonly baseURL: string;

  constructor(baseURL: string, private readonly authToken: string) {
    this.baseURL = baseURL.endsWith("/")
      ? baseURL.substring(0, baseURL.length - 1)
      : baseURL;
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

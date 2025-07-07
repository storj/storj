# Using WebAssembly in Storj

In order to use the uplink library from the browser, we can compile the uplink library to WebAssembly (wasm).

### Setup

To generate wasm code that can create access grants in the web browser, run the following from the storj/wasm directory:
```
$ GOOS=js GOARCH=wasm go build -o access.wasm storj.io/storj/satellite/console/wasm
```

The `access.wasm` code can then be loaded into the browser in a script tag in an html page. Also needed is a JavaScript support file which ships with golang.

To copy the JavaScript support file, run:
```
$ cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" .
```
Ref: [Golang WebAssembly docs](https://github.com/golang/go/wiki/WebAssembly)

The HTML file should include the following:
```
<script type="text/javascript" src="/path/to/wasm_exec.js"></script>
<script>
    const go = new Go();
    WebAssembly.instantiateStreaming(
        fetch("/path/to/access.wasm"), go.importObject).then(
        (result) => {
            go.run(result.instance);
    });
</script>
```

Additionally, the HTTP `Content-Security-Policy (CSP) script-src` directive will need to be modified to allow wasm code to be executed.

See: [WebAssembly Content Security Policy docs](https://github.com/WebAssembly/content-security-policy/blob/master/proposals/CSP.md)

### Usage

#### function newPermission

```js
function newPermission()
```
- **Returns:**

    ```js
    {
        value: {
                AllowDownload: false,
                AllowUpload: false,
                AllowDelete: false,
                AllowList: false,
                NotBefore: "0001-01-01T00:00:00Z",
                NotAfter: "0001-01-01T00:00:00Z",
               },
        error: ""
    }
    ```

- **Usage:**

    newPermission creates a new Permission object with all available permission settings set to default value.

- **Example:**

    ```js
        var permission = newPermission().value;
        permission.AllowedDownload = true;
    ```


### function setAPIKeyPermission

```js
function setAPIKeyPermission(apiKey, buckets, permission)

```

- **Arguments**
    Accepts three arguments: `apiKey`, `buckets` and `permission`

    - **apiKey**
        - **Type:** `String`
        - **Details:**
            This parameter is required

    - **buckets**
        - **Type:** `Array`
        - **Details:**
            An array of bucket names that restrict the api key to only contain enough information to allow access to just those buckets.
            If no bucket names are provided, meaning an empty array, then all buckets are allowed.
            This parameter is required.

    - **permission**
        - **Type:** `Object`
        - **Details:**
            An object that defines what actions can be used for a given api key.
            It should be constructed by calling `newPermission`
            See also: https://github.com/storj/uplink/blob/b8e0f0a90665143a8ce975d92530737130874f5a/access.go#L46
            This parameter is required.

- **Returns**
    ```js
    {
        value: "restricted-api-key",
        error: ""
    }
    ```
    - if an error message is returned, `value` will be set to an empty string.

- **Usage**
    Creates a new api key with specific permissions.

- **Example**
    ```js
        var apiKey = "super-secret-key";
        var buckets = ["test-bucket"];
        var permission = newPermission().value
        permission.allowUpload = true
        var restrictedAPIKey = setAPIKeyPermission(apiKey, buckets, permission)
    ```

### function generateAccessGrant

```js
function generateAccessGrant(satelliteNodeURL, apiKey, encryptionPassphrase, projectSalt, encryptPath)

```

- **Arguments**
    Accepts five arguments: `satelliteNodeURL`, `apiKey`, `encryptionPassphrase`, `projectSalt` and `encryptPath`

    - **satelliteNodeURL**
        - **Type:** `String`
        - **Details:**
            A string that contains satellite node id and satellite address.
            Example: 12tDhBcuMevundiuZPQJd613iW5vCdFtkRDBjBEfjdVtv1hbfCL@127.0.0.1:10000
            This parameter is required

    - **apiKey**
        - **Type:** `String`
        - **Details:**
            This parameter is required

    - **encryptionPassphrase**
        - **Type:** `String`
        - **Details:**
            A string that's used for encryption.
            This parameter is required.

    - **projectSalt**
        - **Type:** `String`
        - **Details:**
            A project-based salt for determinitic key derivation.
            Currently it's referring to a project ID. However, it might change in the future to have more randomness.
            This parameter is required.
    - **encryptPath**
       - **Type:** `Boolean`
       - **Details:**
           Whether path encryption is enabled for the project.
           Giving `false` makes it so that object keys created with the generated access are not encrypted.
    

- **Returns**
    ```js
    {
        value: "access-grant",
        error: ""
    }
    ```
    - if an error message is returned, `value` will be set to an empty string.

- **Usage**
    Creates a new api key with specific permissions.

- **Example**
    ```js
        var satelliteNodeURL = "12tDhBcuMevundiuZPQJd613iW5vCdFtkRDBjBEfjdVtv1hbfCL@127.0.0.1:10000"
        var apiKey = "super-secret-key";
        var passphrase = "123";
        var projectID = "project-id"
        var encryptPath = true
        var result = generateAccessGrant(satelliteNodeURL, apiKey, passphrase, projectID, encryptPath);
        if (result.err != "") {
            // something went wrong
        }
        var access = result.value
    ```

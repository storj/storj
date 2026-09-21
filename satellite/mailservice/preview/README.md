# Email preview

From the repository root:

```sh
go run ./satellite/mailservice/preview
```

Open http://127.0.0.1:8025. Select an email, a viewport width, and HTML or
generated plain text. Templates render locally with sample data; no satellite,
SMTP service, or frontend build is required, and no email is sent.

The page checks for changes every second. Saving any HTML template, including
shared `_head`, `_header`, or `_footer` partials, reloads the selected preview.
New and deleted templates update the selector. Template errors appear in the
preview and recover after the next valid save. Selection and width stay in the
page URL, so previews can be bookmarked.

Plain text is generated from the current HTML using the same converter as
`go generate ./satellite/mailservice/`. Previewing does not rewrite the committed
`.txt` files; run that command when you want to update them.

## Sample data and options

To change message fields, optional links, or branding, copy the provided fixture:

```sh
cp satellite/mailservice/preview/sample.json /tmp/email-preview.json
go run ./satellite/mailservice/preview -data /tmp/email-preview.json
```

The JSON file replaces the default fixture and is also watched for edits. For
example, change `Data.EmailNumber`, `Data.Days`, `Data.IsFree`, or `Data.IsNFR`
to preview other conditional branches. Missing fields are reported as errors.
Numeric fields must be integers to support Go template comparisons.

Use `-addr 127.0.0.1:8026` to change the port, or `-dir /path/to/static/emails`
to preview a different template directory. Images are served from its parent
`static` directory. Keep the server bound to localhost for local development.

The preview uses browser rendering, which can differ from native email clients.

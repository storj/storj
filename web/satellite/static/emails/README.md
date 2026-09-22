# Email templates

Each named HTML file is one email. Keep its wording, links and conditional
message branches here; reuse the shared layout and styles for presentation.
The satellite loads all HTML files together using Go's `html/template`, so
partials work without a separate build step. A shared `dict` function supplies
arguments to self-contained components.

## Structure

- `_head.html`: document metadata, client-specific CSS and the shared
  responsive stylesheet. Only rules used by these emails are included.
- `_header.html`: logo and outer email wrapper.
- `_layout.html`: body, hidden inbox preheader, content/footer containers,
  heading, divider, verification code and button components.
- `_styles.html`: repeated inline typography, table and button styles. Inline
  styles remain important for email clients that strip document stylesheets.
- `_footer.html`: optional social links, company/address and navigation, using
  `social-icon` and `footer-link` components; closes
  the footer container, outer wrapper and document.

A typical email follows this order:

```html
{{ template "head" . }}
<title>Account update</title>
{{ template "body-start" "standard" }}
{{ template "preheader" "Your account has been updated." }}
{{ template "header" . }}
{{ template "content-start" "standard" }}
{{ template "heading" (printf "Your %s account" .BrandName) }}
<tr>
  <td align="left" style="padding:0;Margin:0;padding-top:10px;padding-bottom:10px">
    <p style="{{ template "style-copy" }}">
      Your account has been updated.
    </p>
  </td>
</tr>
{{ template "content-end" }}
{{ template "footer-start" }}
{{ template "footer" . }}
```

Keep content rows between `content-start` and `content-end`. Optional footer
copy goes between `footer-start` and `footer`. The shared fragments deliberately
open and close tables across template boundaries to retain the existing email
client layout. Header markup is a single table; content and footer each use
a padded container cell and a table of rows. Spacing comes from cell padding,
not transparent borders. Place verification-code rows directly in the content
table. `heading` and `divider` are complete rows. Use `button` inside
a cell, passing its label, URL and branding color:

```gotemplate
{{ template "button" (dict "Text" "Reset Password" "URL" .Data.ResetLink "PrimaryColor" .PrimaryColor) }}
```

For a branded label, use `"Text" (printf "Access Your %s Account" .BrandName)`.
Use `button-row` with the same arguments for a complete centered row with
15px vertical padding. Use `button` alone when the surrounding cell needs a
different layout.

`verification-code` is a complete row taking `Code` and `Color`:

```gotemplate
{{ template "verification-code" (dict "Code" .Data.ActivationCode "Color" .PrimaryColor) }}
```

`social-icon` omits its cell when `URL` is empty, and takes `URL`, `Image`
and `Label`, with symmetric spacing so any subset stays centered. `footer-link`
takes `URL`, `Text` and `Separator`; the links wrap as complete items on narrow
screens. The caller guards each link and passes `Separator` for every link after
the first one it renders, so the middot appears only between present links, and
is marked `aria-hidden` so it stays out of the plain text and screen readers.

Footer text and note links consistently use 14px type. Typography comes from
inline styles and stays the same at every viewport width; media queries only
adjust layout. Body copy is 16px/24px, notes are 14px/21px.

The optional `"Simulate" true` argument preserves Welcome's `data-simulate`
attribute. Both production and preview register the same `dict` helper.

Styles such as `style-copy`, `style-link` and `style-note` are inserted inside
`style` attributes. Keep component arguments as ordinary Go values so template
escaping protects text, URLs and colors; do not mark them as trusted HTML.

## Layout variants

Use `standard` unless a message needs an existing variation:

| Fragment | Variant | Purpose |
|---|---|---|
| `body-start` | `fixed-text-size` | Preserve explicit text-size adjustment on account/security emails |
| `content-start` | `bottom-space` | 10px extra bottom spacing |
| `content-start` | `registration` | 40px padding on all sides (standard is 20px vertical, 40px horizontal) |

## Preview and generate

From the repository root:

```sh
go run ./satellite/mailservice/preview
go generate ./satellite/mailservice/
go test ./satellite/mailservice/preview ./satellite/mailservice/htmltext
```

The [preview tool](../../../../satellite/mailservice/preview/README.md) refreshes
when either a message or shared partial changes. It also shows generated plain
text. To regenerate the checked-in `.txt` files, run `go generate` after edits;
do not edit the generated files directly. Layout-only partials become empty
text definitions, while heading and link content remains in the text version.
Those empty definitions still emit their newlines, so the rendered plain text
collapses runs of blank lines before it is sent.

Before changing shared layout or styles, compare all emails at narrow and wide
viewports, including free, paid and NFR message branches. Browser screenshots
verify browser rendering; they do not establish native email-client parity.

## Email-client compatibility

Keep the inline Outlook spacing, line-height and button fallbacks, along with
client-specific CSS for automatic link styling. Go's `html/template` removes
HTML comments, including Outlook conditional comments: placing compatibility
markup inside such comments does not include it in delivered emails. Any new
client-specific fix must be checked in the rendered output and target client.

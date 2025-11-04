# Spanner Key Parser

A command-line tool for parsing Google Cloud Spanner statistics range key start values from the Storj metabase.

## Building

```bash
go build ./cmd/spanner-key-parser
```

## Usage

```bash
./spanner-key-parser '<key>'
```

The tool supports two types of keys:

### Objects Table Keys

Format: `objects(project_id, bucket_name, object_key, version)`

Example:
```bash
./spanner-key-parser 'objects(5\t3w\331>@7\224\0137^Q\277\226=,vivintclips30,\0026\347\343\226E\254wX\325\231\304J\221\016\252N\245\332=\334\034A\343g\014\233o%\235\036\\,\316y\335\202\312\346\275\257\245\370\256>\234/\002\247O\315@\353ln\316S\246\361\254\353O\017\347M_\027\344\036b\370\345\374\004\355\262\264ZX\326\362/\002I\345\"\261\333\356\341\374]>q\261N\235DV\335|R?-h\025\2311'\''o\311\331\265\303\215\\\\\304\225\227\344Z\266\234\327\252M\307\242_\312\310X\r\020\365\242\322g,-1905173148538669962)'
```

Output:
```
Table: objects
  project_id: "5\x093w\xd9>@7\x94\x0b7^Q\xbf\x96=" (hex: 35093377d93e4037940b375e51bf963d)
  bucket_name: vivintclips30
  object_key: ... (hex: ...)
  version: -1905173148538669962
```

### Segments Table Keys

Format: `segments(stream_id, position)`

Example:
```bash
./spanner-key-parser 'segments(\\)X\315x\227\001IG\236\224m\302\025o\204\221,-9223372036854775808)'
```

Output:
```
Table: segments
  stream_id: "\x5c)X\xcdx\x97\x01IG\x9e\x94m\xc2\x15o\x84\x91" (hex: 5c2958cd78970149479e946dc2156f8491)
  position: -9223372036854775808
```

## Key Format

Spanner keys use octal escape sequences for non-printable bytes:
- `\t` - tab character
- `\n` - newline
- `\r` - carriage return
- `\NNN` - octal byte value (e.g., `\002` = byte 0x02)
- `\\` - literal backslash

The tool automatically decodes these escape sequences and displays both human-readable and hexadecimal representations of binary data.

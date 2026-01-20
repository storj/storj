---
name: dbx-regenerate
description: Regenerate DBX code after making changes to .dbx schema files. Runs code generation, shows diff summary, validates compilation, and reports any errors.
allowed_tools: go
allowed_prompts:
  - tool: Bash
    prompt: regenerate DBX code
---

# DBX Regenerate

You are helping regenerate the DBX-generated code after changes to .dbx schema files.

## Overview

DBX (Database Extension) is Storj's schema-first ORM that generates Go code from .dbx schema files. When developers modify .dbx files in `satellite/satellitedb/dbx/`, the generated code in `satellitedb.dbx.go` must be regenerated.

This skill:
1. Runs the DBX code generator
2. Shows a summary of what changed
3. Validates the generated code compiles
4. Reports any errors with helpful context

## Instructions

Follow these steps to regenerate DBX code:

### 1. Check for Modified .dbx Files

First, check which .dbx files have been modified (optional but helpful for context):

```bash
git status satellite/satellitedb/dbx/
```

This helps understand what schema changes triggered the regeneration.

### 2. Run DBX Code Generation

Execute the code generation:

```bash
cd satellite/satellitedb/dbx && go generate
```

This runs the DBX compiler which:
- Parses all .dbx schema files
- Generates SQL for PostgreSQL, CockroachDB, and Spanner
- Produces Go code with type-safe methods
- Creates the ~50,000 line `satellitedb.dbx.go` file

**Expected output**: Should see messages about code generation progress.

### 3. Show Diff Summary

After generation completes, show a summary of what changed:

```bash
git diff --stat satellite/satellitedb/dbx/satellitedb.dbx.go
```

Also show a preview of the changes:

```bash
git diff satellite/satellitedb/dbx/satellitedb.dbx.go | head -100
```

**What to look for**:
- New methods added (e.g., `Create_`, `Get_`, `Update_`, `Delete_` methods)
- Modified method signatures
- New model structs
- Changes to existing queries

### 4. Validate Compilation

Ensure the generated code compiles successfully:

```bash
go build ./satellite/satellitedb/dbx
```

If compilation succeeds, the regeneration was successful.

### 5. Report Results

Provide a clear summary to the user:

**If successful**:
- Confirm DBX regeneration completed
- Summarize what changed (e.g., "Added 3 new methods for the `users` table")
- Show key new/modified methods
- Confirm compilation succeeded
- Suggest next steps (e.g., "You may want to create a database migration for the schema changes")

**If errors occurred**:
- Show the full error output
- Identify which .dbx file has the issue (look for file references in error)
- Explain the error in plain language
- Suggest fixes based on common DBX errors (see below)

## Common DBX Errors and Fixes

### Syntax Error in .dbx File

**Error pattern**: `parse error`, `unexpected token`, `invalid syntax`

**Cause**: Incorrect .dbx syntax (missing commas, invalid field types, etc.)

**Fix**:
- Check the .dbx file line number mentioned in the error
- Verify syntax matches DBX documentation patterns
- Common issues: missing commas between fields, invalid field type names, incorrect query syntax

### Duplicate Model/Query Definition

**Error pattern**: `already defined`, `duplicate definition`

**Cause**: Model or query defined multiple times across .dbx files

**Fix**:
- Search for the duplicate definition across all .dbx files
- Remove or rename the duplicate
- Ensure model names are unique across all .dbx files

### Invalid Field Type

**Error pattern**: `unknown type`, `invalid type`

**Cause**: Using a field type that DBX doesn't recognize

**Valid DBX types**:
- `text`, `blob`, `int`, `int64`, `uint`, `uint64`
- `bool`, `timestamp`, `float64`
- `utimestamp` (microsecond timestamp)

**Fix**: Change the field type to a valid DBX type

### Missing Primary Key

**Error pattern**: `no primary key defined`

**Cause**: Table definition missing a primary key

**Fix**: Add a primary key using `( key <field> )` in the model definition

### Invalid Query Pattern

**Error pattern**: `invalid query`, `unknown read pattern`

**Cause**: Using an incorrect query pattern name

**Valid patterns**:
- `read one`, `read all`, `read first`, `read paged`, `read limitoffset`, `read scalar`
- `update`, `delete`, `create`, `count`

**Fix**: Use a valid query pattern from the list above

### Circular Dependency

**Error pattern**: `circular dependency`, `import cycle`

**Cause**: .dbx files referencing each other in a circular way

**Fix**: Restructure the models to remove circular references

## Post-Regeneration Checklist

After successful regeneration, remind the user to:

1. **Review the diff**: Check that generated changes match expectations
2. **Create migration**: If schema changed, create database migration files
3. **Run linter**: Ensure generated code passes linting
   ```bash
   make llint LINT_TARGET=./satellite/satellitedb/dbx
   ```
4. **Commit changes**: Commit both .dbx changes and generated code together

## Understanding the Generated Code

The `satellitedb.dbx.go` file contains:

- **Model structs**: Go representations of database tables
- **Field types**: Type-safe field constructors (e.g., `User_Id_Field`)
- **CRUD methods**: Auto-generated database operations
- **Backend implementations**: Separate implementations for PostgreSQL, CockroachDB, and Spanner
- **Transaction support**: `WithTx()` methods for transactions
- **Error handling**: Wrapped errors with proper error codes

## Example Workflow

**User modified**: `satellite/satellitedb/dbx/user.dbx` - added `last_login timestamp` field

**Regeneration output**:
```
DBX regenerated successfully!

Changes:
- Modified: User model (added field: last_login)
- New methods: Update_User_LastLogin_By_Id
- Lines changed: +156 -12

Generated methods:
- User.LastLogin field added
- User_LastLogin_Field type added
- Update methods now include last_login in optional fields

Compilation: ✓ Success

Next steps:
1. Review the diff to ensure changes are correct
2. Create a migration to add the last_login column
3. Run tests to ensure compatibility
```

## Notes

- DBX generation can take 10-30 seconds depending on system performance
- The generated file is ~1.9 MB and ~50,000 lines of code
- Always regenerate after modifying ANY .dbx file
- The generator is located in `satellite/satellitedb/dbx/gen/main.go`
- DBX supports PostgreSQL, CockroachDB, and Spanner from the same schema
- Generated code includes monkit instrumentation for metrics
- All errors are wrapped with `errs.Class("satellitedb")`

## Tips

- Use `git diff` to see exactly what changed before committing
- If regeneration seems slow, check for syntax errors first
- Keep .dbx files focused - one file per logical domain (users, projects, etc.)
- Test generated code with all supported backends if possible
- DBX errors are usually specific - read them carefully for the fix

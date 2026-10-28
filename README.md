# 🩹 patcher

A fast, lightweight, and robust CLI tool built in Go to safely apply multi-file code modifications and patches from LLM / AI assistant outputs directly to your project.

---

## 🌟 Why patcher?

Copying code suggestions across multiple files from AI assistants is tedious and error-prone. Manually finding exact lines, applying partial edits, or creating new directory trees slows down development.

`patcher` streamlines this: paste the assistant's output into a text file and run:

```bash
patcher changes.txt
```

Any conversational text, explanations, or markdown wrappers outside the directive tags are automatically ignored.

---

## 🛡️ Key Features & Safety Guarantees

- **No Tag Leakage Guarantee:** Automatically strips accidental nested or duplicated patcher tags from inside replacements and file writes. Directives are **never** accidentally written into user source code.
- **Targeted Region Updates (No Line Guessing):** LLMs no longer need to guess line numbers or reproduce large blocks of code. Target specific regions with:
  - Pattern bounds (`from="..." to="..."`)
  - Function/struct symbols (`symbol="..."`)
  - Wildcard / ellipsis in search blocks (`...`)
  - Line numbers (`lines="45-120"`)
  - Anchor insertions (`after="..."` or `before="..."`)
- **Automatic Line Number Stripping:** If an LLM copies numbered lines from context tools like `textify` (`   45 | func Foo()`), `patcher` automatically scrubs the line numbers and applies clean source code.
- **Code Fence Stripping:** Automatically detects and strips accidental outer code block fences (` ```lang ... ``` `) generated inside `<file>` and `<with>` blocks.
- **Two-Pass Atomic Validation (VFS):** Validates all operations in an in-memory virtual filesystem before touching disk. If any search target or region fails to match, **zero files are touched**.
- **Sequential In-Memory Execution:** Changes run strictly in document order. You can create a file and run subsequent targeted replacements on it within the same patch file.
- **5-Tier Fuzzy Matching for Search Blocks:**
  1. *Exact match*
  2. *Line ending normalization* (`\r\n` vs `\n`)
  3. *Trailing whitespace tolerance*
  4. *Indentation tolerance*
  5. *Ellipsis / wildcard interpolation* (`...`)

---

## 📋 Directive Format Reference

*(Note: In actual patch files, use tags without space after `<`)*

### 1. Write or Overwrite a File: `< file>`

Creates a new file or completely overwrites an existing file. Missing parent directories are created automatically.

```xml
< file path="cmd/app/main.go">
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
< /file>
```

---

### 2. Targeted Region Updates (No Line Guessing!)

#### A. Block Replacement (`from="..." to="..."`)
Replaces everything from the starting pattern to the stopping pattern:

```xml
< replace path="pkg/compiler/typechecker.go" from="case *PromoteExpr:" to="case *">
< with>
case *PromoteExpr:
    val := e.Value
    genPromote(val)
case *
< /with>
< /replace>
```

#### B. Symbol Replacement (`symbol="..."`)
Replaces an entire function, method, struct, or class declaration by name:

```xml
< replace path="pkg/compiler/codegen.go" symbol="inferExpr">
< with>
func inferExpr(ctx *Context, expr Expr) Type {
    return ctx.Resolve(expr)
}
< /with>
< /replace>
```

#### C. Ellipsis / Wildcard Matching (`...`)
Target modifications inside a large block without reproducing the entire block:

```xml
< replace path="pkg/config/config.go">
< search>
type Config struct {
    ...
    Timeout int
    ...
}
< /search>
< with>
type Config struct {
    ...
    Timeout int
    Retries int
    ...
}
< /with>
< /replace>
```

#### D. Anchor Insertions (`after="..."` or `before="..."`)
Inserts new code directly before or after an anchor line:

```xml
< replace path="pkg/api/server.go" after="router := gin.Default()">
< with>
    router.Use(cors.Default())
< /with>
< /replace>
```

#### E. Explicit Line Ranges (`lines="..."`)
When line numbers are known (e.g. from `textify signatures`):

```xml
< replace path="pkg/utils/math.go" lines="45-60">
< with>
func Add(a, b int) int {
    return a + b
}
< /with>
< /replace>
```

---

### 3. Delete a File: `< delete />`

Deletes a file if it exists:

```xml
< delete path="path/to/obsolete_file.go" />
```

---

## 🛠️ Typical Workflow

1. **Ask your assistant** for changes using `<file>`, `<replace>`, or `<delete />` directives.
2. **Save the response** into a file (e.g. `patch.txt`).
3. **(Optional) Dry run**:
   ```bash
   patcher patch.txt --dry-run
   ```
4. **Apply changes**:
   ```bash
   patcher patch.txt
   ```
5. **Inspect & Clean up**:
   ```bash
   git diff
   rm patch.txt
   ```

---

## 📖 CLI Usage & Options

```text
Usage:
  patcher <patch-file.txt> [--dry-run]   Apply patch file
  patcher version                        Show installed version
  patcher help                           Show this help message

Options:
  --dry-run, -n                          Preview changes without writing to disk
  --version, -v                          Show version information
  --help, -h                             Show this help message
```

---

## 📄 License

Distributed under the MIT License.
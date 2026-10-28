
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

- **Two-Pass Atomic Validation (VFS):** Validates all operations in an in-memory virtual filesystem before touching disk. If any `<search>` block fails to match or a file is missing, **zero files are touched**.
- **Sequential In-Memory Execution:** Changes run strictly in document order. You can create a file and run subsequent `<replace>` blocks on it within the same patch file.
- **Anti-Corruption Tag Shield:** Detects unclosed tags and nested directives immediately, preventing runaway tags from swallowing subsequent edits.
- **4-Tier Fuzzy Matching for `<search>`:**
  1. *Exact match*
  2. *Line ending normalization* (`\r\n` vs `\n`)
  3. *Trailing whitespace tolerance* (ignores trailing spaces/tabs on lines)
  4. *Indentation tolerance* (matches blocks regardless of tab vs. space indentation differences)
- **Helpful Diagnostics:** If a search target does not match, `patcher` provides a contextual mismatch hint showing the expected line vs. actual content.
- **Code Fence Stripping:** Automatically detects and strips accidental outer ` ```lang ... ``` ` wrappers generated inside `<file>` blocks.
- **Pasted Text Normalization:** Automatically fixes smart/curly quotes (`“`, `”`, `‘`, `’`) and decodes HTML entities (`&lt;`, `&gt;`) from rich text.
- **Automatic Directory Creation:** Creates any missing parent directories on demand when writing files.

---

## 🚀 Installation

### Prerequisites
- Go 1.22+ (or compatible toolchain)

### Build and Install

```bash
# Clone the repository
git clone https://github.com/johnesleyer/patcher.git
cd patcher

# Build binary across all package files
go build -o patcher .

# Install to your local PATH (Linux / macOS)
mv patcher ~/.local/bin/

# Or system-wide
sudo mv patcher /usr/local/bin/
```

Verify the installation:
```bash
patcher --help
```

---

## 📋 Directive Format Reference

`patcher` scans text for three XML-like tags. Multiple operations can be included in a single file in any order.

### 1. Write or Overwrite a File: `<file>`

Creates a new file or completely overwrites an existing file. Missing parent directories are created automatically.

```xml
<file path="path/to/file.ext">
package main

func main() {
    println("Hello, World!")
}
</file>
```

---

### 2. Targeted Search & Replace: `<replace>`

Searches for an exact block of code inside an existing file and replaces it.

```xml
<replace path="path/to/file.ext">
<search>
func OldImplementation() {
    // legacy logic
}
</search>
<with>
func NewImplementation() {
    // improved logic
}
</replace>
```

---

### 3. Delete a File: `<delete />`

Deletes a file if it exists.

```xml
<delete path="path/to/unneeded_file.go" />
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
Usage: patcher <patch-file.txt> [--dry-run]
       patcher --version

Options:
  --dry-run, -n    Preview changes without writing to disk
  --version, -v    Show version information
  --help, -h       Show this help message
```

---

## 📄 License

MIT License.


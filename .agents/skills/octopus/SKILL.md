```markdown
# octopus Development Patterns

> Auto-generated skill from repository analysis

## Overview
This skill teaches the core development patterns and conventions used in the `octopus` Go codebase. You'll learn how to structure files, write and export code, manage imports, follow commit message conventions, and write tests. These patterns help maintain code consistency and facilitate collaboration.

## Coding Conventions

### File Naming
- Use **camelCase** for file names.
  - Example: `myHandler.go`, `userService.go`

### Import Style
- Use **relative imports** within the project.
  - Example:
    ```go
    import "../utils"
    ```

### Export Style
- Use **named exports** for functions, types, and variables.
  - Example:
    ```go
    package mypackage

    func MyFunction() {
        // ...
    }
    ```

### Commit Messages
- Use **conventional commit** format.
- Allowed prefixes: `feat`, `fix`
- Example:
  ```
  feat: add user authentication middleware
  fix: correct typo in error message
  ```

## Workflows

### Feature Development
**Trigger:** When adding a new feature  
**Command:** `/feature`

1. Create a new branch for your feature.
2. Write code in camelCase-named files.
3. Use relative imports for internal dependencies.
4. Export new functions/types with named exports.
5. Commit using the `feat:` prefix and a concise message.
6. Open a pull request for review.

### Bug Fixing
**Trigger:** When fixing a bug  
**Command:** `/bugfix`

1. Create a new branch for the bug fix.
2. Update code, maintaining file naming and import conventions.
3. Use the `fix:` prefix in your commit message.
4. Add or update tests as needed.
5. Open a pull request referencing the bug.

### Testing
**Trigger:** Before merging or to verify changes  
**Command:** `/test`

1. Locate or create test files matching the `*.test.*` pattern.
2. Write tests for new or updated code.
3. Run tests using Go's built-in testing tools (e.g., `go test`).
4. Ensure all tests pass before merging.

## Testing Patterns

- Test files follow the `*.test.*` naming pattern (e.g., `userService.test.go`).
- Testing framework is unspecified; use Go's standard testing package.
- Example test file:
  ```go
  package mypackage

  import "testing"

  func TestMyFunction(t *testing.T) {
      result := MyFunction()
      if result != expected {
          t.Errorf("expected %v, got %v", expected, result)
      }
  }
  ```

## Commands
| Command   | Purpose                                 |
|-----------|-----------------------------------------|
| /feature  | Start a new feature development workflow |
| /bugfix   | Start a bug fixing workflow             |
| /test     | Run or write tests                      |
```
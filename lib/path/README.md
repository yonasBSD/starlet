# path

`path` provides functions to manipulate directories and file paths. It is inspired by `pathlib` module from Mojo.

## Functions

### `abs(path) string`

Returns an absolute representation of path. If the path is not absolute it will be joined with the current working directory to turn it into an absolute path. The absolute path name for a given file is not guaranteed to be unique.

#### Parameters

| name   | type     | description                                        |
|--------|----------|----------------------------------------------------|
| `path` | `string` | The file path to be converted to its absolute form |

#### Examples

**String**

Convert a relative path to an absolute path.

```python
load("path", "abs")
p = abs('.')
print(p)
# Output: '/current/absolute/path'
```

### `join(path, *paths) string`

Joins one or more path elements into a single path intelligently, separating them with an OS specific separator. Empty elements are ignored.

#### Parameters

| name       | type     | description                    |
|------------|----------|--------------------------------|
| `paths...` | `string` | The path elements to be joined |

#### Examples

**String**

Join multiple path parts.

```python
load("path", "join")
p = join('a', 'b', 'c')
print(p)
# Output: 'a/b/c'
```

### `exists(path) bool`

Returns true if the path exists.

#### Parameters

| name   | type     | description            |
|--------|----------|------------------------|
| `path` | `string` | The path to be checked |

#### Examples

**String**

Check if a path exists.

```python
load("path", "exists")
p = exists('path_test.go')
print(p)
# Output: True
```

### `is_file(path) bool`

Returns true if the path exists and is a file.

#### Parameters

| name   | type     | description            |
|--------|----------|------------------------|
| `path` | `string` | The path to be checked |

#### Examples

**String**

Check if a path is a file.

```python
load("path", "is_file")
p = is_file('path_test.go')
print(p)
# Output: True
```

### `is_dir(path) bool`

Returns true if the path exists and is a directory.

#### Parameters

| name   | type     | description            |
|--------|----------|------------------------|
| `path` | `string` | The path to be checked |

#### Examples

**String**

Check if a path is a directory.

```python
load("path", "is_dir")
p = is_dir('.')
print(p)
# Output: True
```

### `is_link(path) bool`

Returns true if the path exists and is a symbolic link.

#### Parameters

| name   | type     | description            |
|--------|----------|------------------------|
| `path` | `string` | The path to be checked |

#### Examples

**String**

Check if a path is a symbolic link.

```python
load("path", "is_link")
p = is_link('link_to_path_test.go')
print(p)
# Output: False
```

### `listdir(path, recursive=False) []string`

Returns a list of directory contents.

#### Parameters

| name        | type     | description                        |
|-------------|----------|------------------------------------|
| `path`      | `string` | The directory path                 |
| `recursive` | `bool`   | If true, list contents recursively |

#### Examples

**String**

List directory contents.

```python
load("path", "listdir")
p = listdir('.')
print(p)
# Output: ['file1', 'file2', ...]
```

### `getcwd() string`

Returns the current working directory.

#### Examples

**String**

Get the current working directory.

```python
load("path", "getcwd")
p = getcwd()
print(p)
# Output: '/current/directory'
```

### `chdir(path)`

Changes the current working directory.

#### Parameters

| name   | type     | description                           |
|--------|----------|---------------------------------------|
| `path` | `string` | The path to the new current directory |

#### Examples

**String**

Change the current working directory.

```python
load("path", "chdir")
chdir('/new/directory')
# Current directory is now '/new/directory'
```
# gag the tag explorer

A little CLI tool for exploring my own writing, structured with the [UM schema](https://github.com/brtholomy/um).

```sh
gag foo
```

Response in the ./testdata dir, formatted like TOML:

```sh
[files]
01.foo.md

[tags]
foo

[adjacencies]
sot

[sums]
files = 1
adjacencies = 1
```

One of the most useful flags is `--pipe`:

```sh
gag --query foo --pipe
```

Which gives a simple list of files ready to be piped to `cat`:

```sh
gag --query foo --pipe | xargs cat > /tmp/foo.md
```

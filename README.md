# gag the tag explorer

A little CLI tool for exploring my own writing, structured with the [UM schema](https://github.com/brtholomy/um).

```sh
gag foo --verbose
```

Response in the ./testdata dir, formatted like TOML:

```sh
[files]
01.foo.md

[tags]
foo

[adjacencies]
bar

[sums]
files = 1
adjacencies = 1
```

And plain:


```sh
gag bar
```

Output is ready to pipe:

```sh
01.foo.md
02.foo.md
03.bar.md
```

Like this:

```sh
gag foo | xargs cat > /tmp/foo.md
```

Also accepts piped input from stdin:

```sh
ls *foo*md | gag bar
```

And can therefore be chained:

```sh
gag foo | gag bar --invert
```

Tags can also be intersected:

```sh
gag foo+bar
```

See `--help` for all flags.

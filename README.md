# subpath-serve

A small server used to serve text files from my dotfiles (though it could be used to serve anything).

Any request to `/...` tries to match against some file basepath in `./serve`.

A request to the base path (`/`) without anything else returns a newline delimited list of everything in the `./serve` folder.

Does not build an index at build/initial server start, so the `./serve` folder can be modified while the server is running to change results; each request searches the folder for the query.

Appending `?dark` to the end of a URL converts a request to an HTML response with a dark theme.

Example Requests:

- <https://sean.fish/d/>
- <https://sean.fish/d/rc.conf/>
- <https://sean.fish/d/rc.conf/?dark>

### matching strategy

An example of how this matches. If the files in `./serve` are:

```
./folder1/a
./folder2/a
./folder3/b
```

| Request to | Resolves to |
| ---------- | ----------- |
| /a         | ./folder1/a |
| /b         | ./folder3/b |
| /folder2/a | ./folder2/a |

It matches `./folder1/a` just because thats the one it found first, if there's a possibility of a conflict, its better to provide a unique subpath.

### Run

```sh
Usage of subpath-serve:
  -folder string
    	path to serve subpath-serve on (default "./serve")
  -port int
    	port to serve subpath-serve on (default 8050)
```

```
git clone "https://gitlab.com/seanbreckenridge/dotfiles.git" "./serve"
subpath-serve
```

```
curl localhost:8050/rc.conf
```

The response contains the `X-FilePath` header, which includes the full path to the matched file.

### Install

Install `golang`.

You can clone and run `go build`, or:

```
go get -v -u "gitlab.com/seanbreckenridge/subpath-serve"
```

which downloads, builds and puts the binary on your `$GOBIN`.

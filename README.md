# sonarr-config-tool

Tool to import series to Sonarr from a media directory

## Development

### Prerequisites
- [pre-commit](https://pre-commit.com/#install)
- [golang](https://golang.org/doc/install#install)
- [golint](https://github.com/golang/lint#installation)
- [docker](https://docs.docker.com/get-docker/)
- [shellcheck](https://www.shellcheck.net/)
- [coreutils](https://formulae.brew.sh/formula/coreutils) required on macOS (due to use of realpath).

### Configurations

- Configure pre-commit hooks
```sh
pre-commit install
```

### Tests

#### App
```sh
go test ./...
```

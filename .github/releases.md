# Release preparation

1. Run linters, fix all simple warnings. If the behavior is intentional - add
`nolint` comment and explanation. If the warning is non-trviail to fix - open
an issue.
```
golangci-lint run
```

2. Run unit tests suite. Verify that all disabled tests are not related to
   serious problems and have corresponding issue open.
```
go test ./...
```

3. Run integration tests suite. Verify that all disabled tests are not related
   to serious problems and have corresponding issue open.
```
cd tests/
./run.sh
```

4. Write release notes.

5. Create PGP-signed Git tag and push it to GitHub (do not create a "release"
   yet).

5. Use environment configuration from maddy-repro bundle
   (https://foxcpp.dev/maddy-repro) to build release artifacts.

6. Create detached PGP signatures for artifacts using key
   3197BBD95137E682A59717B434BB2007081396F4.

7. Create sha256sums file for artifacts.

8. Create release on GitHub using the same text for
   release notes. Attach signed artifacts and sha256sums file.

9. Build the Docker container and push it to hub.docker.com.

10. Post a message on the sr.ht mailing list.

# AsianAmericans.wiki

## About

Source code for AsianAmericans.wiki.

## Running Locally

To run tailwind watcher:

```shell
 npx tailwindcss -i functions/api/server/public/static/input.css -o ./functions/api/server/public/static/output.css --watch
```

To run the emulators:

```shell
firebase emulators:start --only "auth,firestore"
```

To run the backend locally (emulators required):

```shell
go run functions/api/cmd/main.go --local
```

Or use air (for hot reload)

```shell
air
```

## Formatting

All source code should be formatted with prettier for consistency.

```shell
npm run prettier
```

## Search

There is a Firestore -> Algolia extension used for the search index.

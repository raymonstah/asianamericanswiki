# AsianAmericans.wiki

## About

Source code for AsianAmericans.wiki.

## Contributing

I could use some UI help. If you have experience in Tailwind or Svelte.js, feel
free to make changes an open up a pull request.

## Running Locally

To run the UI locally:

```shell
npm run dev
```

To run the emulators:

```shell
firebase emulators:start --only "auth,firestore"
```

To run the backend locally (emulators required):

```shell
go run functions/api/cmd/main.go --local
```

## Formatting

All source code should be formatted with prettier for consistency.

```shell
npm run prettier
```

## Search

There is a Firestore -> Algolia extension used for the search index.

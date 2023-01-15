# AsianAmericans.wiki

## About

AsianAmericans.wiki is generated from a website generator,
[Hugo](https://gohugo.io/).

## Contributing

If you want to contribute to the list of Asian Americans, you can fork this repo
and add a new human to `content/humans` . Please follow the same conventions as
the existing humans.

An easy way to do this is to use Hugo's archetype:

```shell
hugo new humans/YOUR_HUMAN_HERE/index.md
```

To preview the site locally with your changes:

```shell
hugo serve
```

and then navigate to http://localhost:1313 to verify your changes.

Once you're satisfied with your changes, you can submit a pull request. As soon
as your pull request gets merged, your changes will go live in production.

> :warning: I'm not accepting pull requests related to the website itself.
> Content only please!

## Formatting

All source code should be formatted with prettier for consistency.

```shell
npm run prettier
```

## Search

There is a Firestore -> Algolia extension used for the search index.

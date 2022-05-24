# Contributor

Contributor is used for the /contribute page. Once a user POSTs an Asian
American they want to contribute, a GCP Cloud Function is invoked and will
create a pull request.

```shell
gcloud beta functions deploy go-http-function \
--gen2 \
--runtime go116 \
--trigger-http \
--entry-point Handle \
--source . \
--allow-unauthenticated
```

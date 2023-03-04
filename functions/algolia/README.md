# Algolia Search Integration

Deploy with:

```shell
gcloud functions deploy algolia-sync --entry-point HelloFirestore --runtime go119 --trigger-event "providers/cloud.firestore/eventTypes/document.write" --trigger-resource "projects/asianamericans-wiki/databases/(default)/documents/humans/{human}" --memory 128mb --max-instances 1 --set-secrets=ALGOLIA_API_KEY=projects/424340922093/secrets/firestore-algolia-search-ALGOLIA_API_KEY:latest --set-env-vars ALGOLIA_APP_ID=I3Z39HZCDT --retry
```

todo: not part of CI pipeline yet.

# Twitter Follower Integration

Deploy with:

```shell
gcloud functions deploy twitter-sync --entry-point TwitterFollow --runtime go119 --trigger-event "providers/cloud.firestore/eventTypes/document.write" --trigger-resource "projects/asianamericans-wiki/databases/(default)/documents/humans/{human}" --memory 128mb --max-instances 1 \
--set-secrets=TWITTER_CONSUMER_KEY=projects/424340922093/secrets/firestore-twitter-follow-TWITTER_CONSUMER_KEY:latest \
--set-secrets=TWITTER_CONSUMER_SECRET=projects/424340922093/secrets/firestore-twitter-follow-TWITTER_CONSUMER_SECRET:latest \
--set-secrets=TWITTER_ACCESS_TOKEN=projects/424340922093/secrets/firestore-twitter-follow-TWITTER_ACCESS_TOKEN:latest \
--set-secrets=TWITTER_ACCESS_SECRET=projects/424340922093/secrets/firestore-twitter-follow-TWITTER_ACCESS_SECRET:latest \
--retry
```

todo: not part of CI pipeline yet.

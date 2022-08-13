# API

## Start emulators

```bash
 firebase emulators:start --only "auth,firestore"
```

## Deploying Google Cloud Run

```bash
 gcloud run deploy api --max-instances 1 --timeout 10 --region us-central1 --memory 128Mi
```

## Deploying Firestore indices

```bash
firebase deploy --only firestore:indexes
```

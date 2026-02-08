# API

## Start emulators

```bash
 firebase emulators:start --only "auth,firestore"
```

## Environment Variables

The following environment variables are required for the API server:

- `FIREBASE_API_KEY`: The API key for your Firebase project.
- `FIREBASE_AUTH_DOMAIN`: The auth domain for your Firebase project.
- `FIREBASE_PROJECT_ID`: The project ID for your Firebase project.
- `FIREBASE_STORAGE_BUCKET`: The storage bucket for your Firebase project.
- `FIREBASE_MESSAGING_SENDER_ID`: The messaging sender ID for your Firebase project.
- `FIREBASE_APP_ID`: The app ID for your Firebase project.
- `FIREBASE_MEASUREMENT_ID`: The measurement ID for your Firebase project.
- `XAI_API_KEY`: The API key for xAI (optional, for image generation).

## Deploying Google Cloud Run

```bash
 gcloud run deploy api --max-instances 1 --timeout 10 --region us-central1 --memory 128Mi
```

## Deploying Firestore indices

```bash
firebase deploy --only firestore:indexes
```
